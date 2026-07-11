package contract

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestValidatorExecutesCustomRulesAfterSchemaValidation(t *testing.T) {
	handler := testRequestContractHandler(t, validationCatalogRuntimeSpec)
	tests := []struct {
		name            string
		method          string
		target          string
		body            string
		envelopeMessage string
		want            map[string]any
	}{
		{
			name:            "notBlank",
			method:          http.MethodPost,
			target:          "/profiles",
			body:            `{"name":"  "}`,
			envelopeMessage: "请求体参数校验失败",
			want: map[string]any{
				"location": "body", "field": "name", "path": "name", "rule": "notBlank", "message": "姓名不能为空",
			},
		},
		{
			name:            "containsLetterAndDigit",
			method:          http.MethodPost,
			target:          "/profiles",
			body:            `{"name":"alice","password":"abcdefgh"}`,
			envelopeMessage: "请求体参数校验失败",
			want: map[string]any{
				"location": "body", "field": "password", "path": "password", "rule": "containsLetterAndDigit", "message": "password 至少包含字母和数字",
			},
		},
		{
			name:            "notAfter",
			method:          http.MethodGet,
			target:          "/profiles?createdAtFrom=2026-01-02T00:00:00&createdAtTo=2026-01-01T00:00:00",
			envelopeMessage: "请求参数校验失败",
			want: map[string]any{
				"location": "query", "field": "createdAtFrom", "path": "createdAtFrom", "rule": "notAfter", "message": "createdAtFrom 不能晚于 createdAtTo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := customRuleTestRequest(tt.method, tt.target, tt.body)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			responseBody := assertContractError(t, recorder, http.StatusBadRequest, tt.envelopeMessage)
			assertSingleViolation(t, responseBody, tt.want)
		})
	}
}

func TestRequestValidatorAggregatesCustomRuleViolationsInStableOrder(t *testing.T) {
	handler := testRequestContractHandler(t, validationCatalogRuntimeSpec)
	request := customRuleTestRequest(
		http.MethodPost,
		"/profiles",
		`{"name":"  ","password":"abcdefgh"}`,
	)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	body := assertContractError(t, recorder, http.StatusBadRequest, "请求体参数校验失败")
	violations := body["data"].(map[string]any)["violations"].([]any)
	if len(violations) != 2 {
		t.Fatalf("violations count = %d, want 2: %#v", len(violations), violations)
	}
	want := []struct {
		field   string
		rule    string
		message string
	}{
		{field: "name", rule: "notBlank", message: "姓名不能为空"},
		{field: "password", rule: "containsLetterAndDigit", message: "password 至少包含字母和数字"},
	}
	for index, expected := range want {
		violation := violations[index].(map[string]any)
		if violation["field"] != expected.field || violation["rule"] != expected.rule || violation["message"] != expected.message {
			t.Fatalf("violations[%d] = %#v, want field=%s rule=%s message=%s", index, violation, expected.field, expected.rule, expected.message)
		}
	}
}

func TestRequestValidatorAllowsValidCustomRuleValues(t *testing.T) {
	handler := testRequestContractHandler(t, validationCatalogRuntimeSpec)
	tests := []struct {
		name   string
		method string
		target string
		body   string
	}{
		{name: "valid field rules", method: http.MethodPost, target: "/profiles", body: `{"name":"alice","password":"Password1"}`},
		{name: "ordered range", method: http.MethodGet, target: "/profiles?createdAtFrom=2026-01-01T00:00:00&createdAtTo=2026-01-02T00:00:00"},
		{name: "equal range", method: http.MethodGet, target: "/profiles?createdAtFrom=2026-01-01T00:00:00&createdAtTo=2026-01-01T00:00:00"},
		{name: "incomplete range", method: http.MethodGet, target: "/profiles?createdAtFrom=2026-01-01T00:00:00"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := customRuleTestRequest(tt.method, tt.target, tt.body)
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			if recorder.Code != http.StatusNoContent {
				t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusNoContent, recorder.Body.String())
			}
		})
	}
}

func TestRequestValidatorRunsSchemaValidationBeforeCustomRules(t *testing.T) {
	handler := testRequestContractHandler(t, validationCatalogRuntimeSpec)
	request := customRuleTestRequest(http.MethodPost, "/profiles", `{"name":" "}`)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	body := assertContractError(t, recorder, http.StatusBadRequest, "请求体参数校验失败")
	assertSingleViolation(t, body, map[string]any{
		"location": "body",
		"field":    "name",
		"path":     "name",
		"rule":     "minLength",
		"message":  "姓名至少 2 个字符",
	})
}

func TestRequestValidatorSkipsCustomRulesWhenRequestValidationIsDisabled(t *testing.T) {
	spec, err := LoadSpec(writeSpec(t, validationCatalogRuntimeSpec))
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	validator, err := NewRequestValidator(spec, WithRequestValidation(false))
	if err != nil {
		t.Fatalf("new request validator: %v", err)
	}
	handler := validator.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := customRuleTestRequest(http.MethodPost, "/profiles", `{"name":"  "}`)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
}

func TestRequestValidatorPreservesBodyAfterCustomRuleEvaluation(t *testing.T) {
	spec, err := LoadSpec(writeSpec(t, validationCatalogRuntimeSpec))
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	validator, err := NewRequestValidator(spec)
	if err != nil {
		t.Fatalf("new request validator: %v", err)
	}
	const rawBody = `{"name":"alice","password":"Password1"}`
	handler := validator.Middleware(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		body, readErr := io.ReadAll(request.Body)
		if readErr != nil {
			t.Fatalf("read downstream body: %v", readErr)
		}
		if string(body) != rawBody {
			t.Fatalf("downstream body = %q, want %q", body, rawBody)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	request := customRuleTestRequest(http.MethodPost, "/profiles", rawBody)
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
}

func TestFieldCustomRulePassesUsesWhitespaceAndASCIICompositionSemantics(t *testing.T) {
	tests := []struct {
		name  string
		rule  string
		value string
		want  bool
	}{
		{name: "notBlank rejects ASCII whitespace", rule: customRuleNotBlank, value: " \t\n", want: false},
		{name: "notBlank rejects Unicode whitespace", rule: customRuleNotBlank, value: "\u3000", want: false},
		{name: "notBlank accepts text", rule: customRuleNotBlank, value: " eventhub ", want: true},
		{name: "composition rejects letters only", rule: customRuleContainsLetterAndDigit, value: "abcdefgh", want: false},
		{name: "composition rejects digits only", rule: customRuleContainsLetterAndDigit, value: "12345678", want: false},
		{name: "composition accepts ASCII letter and digit", rule: customRuleContainsLetterAndDigit, value: "密码A1", want: true},
		{name: "composition does not treat non ASCII classes as ASCII", rule: customRuleContainsLetterAndDigit, value: "密码１", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fieldCustomRulePasses(tt.rule, tt.value); got != tt.want {
				t.Fatalf("fieldCustomRulePasses(%q, %q) = %t, want %t", tt.rule, tt.value, got, tt.want)
			}
		})
	}
}

func customRuleTestRequest(method, target, body string) *http.Request {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	request := httptest.NewRequest(method, target, reader)
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	return request
}
