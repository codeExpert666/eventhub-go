package contract

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestValidatorUsesValidationCatalogMessages(t *testing.T) {
	handler := testRequestContractHandler(t, validationCatalogRuntimeSpec)
	tests := []struct {
		name   string
		method string
		target string
		body   string
		want   map[string]any
	}{
		{
			name:   "body required",
			method: http.MethodPost,
			target: "/profiles",
			body:   `{}`,
			want: map[string]any{
				"location": "body", "field": "name", "path": "name", "rule": "required", "message": "姓名不能为空",
			},
		},
		{
			name:   "body minLength",
			method: http.MethodPost,
			target: "/profiles",
			body:   `{"name":"a"}`,
			want: map[string]any{
				"location": "body", "field": "name", "path": "name", "rule": "minLength", "message": "姓名至少 2 个字符",
			},
		},
		{
			name:   "body maxLength",
			method: http.MethodPost,
			target: "/profiles",
			body:   `{"name":"abcdef"}`,
			want: map[string]any{
				"location": "body", "field": "name", "path": "name", "rule": "maxLength", "message": "姓名不能超过 5 个字符",
			},
		},
		{
			name:   "body pattern",
			method: http.MethodPost,
			target: "/profiles",
			body:   `{"name":"alice","code":"lower"}`,
			want: map[string]any{
				"location": "body", "field": "code", "path": "code", "rule": "pattern", "message": "code 只能使用大写字母",
			},
		},
		{
			name:   "body format",
			method: http.MethodPost,
			target: "/profiles",
			body:   `{"name":"alice","createdAt":"not-a-date-time"}`,
			want: map[string]any{
				"location": "body", "field": "createdAt", "path": "createdAt", "rule": "format", "message": "createdAt 格式不合法",
			},
		},
		{
			name:   "body enum",
			method: http.MethodPost,
			target: "/profiles",
			body:   `{"name":"alice","status":"DELETED"}`,
			want: map[string]any{
				"location": "body", "field": "status", "path": "status", "rule": "enum", "message": "status 只能是 ENABLED 或 DISABLED",
			},
		},
		{
			name:   "body minimum",
			method: http.MethodPost,
			target: "/profiles",
			body:   `{"name":"alice","score":0}`,
			want: map[string]any{
				"location": "body", "field": "score", "path": "score", "rule": "minimum", "message": "score 不能小于 1",
			},
		},
		{
			name:   "body maximum",
			method: http.MethodPost,
			target: "/profiles",
			body:   `{"name":"alice","score":11}`,
			want: map[string]any{
				"location": "body", "field": "score", "path": "score", "rule": "maximum", "message": "score 不能大于 10",
			},
		},
		{
			name:   "query minimum",
			method: http.MethodGet,
			target: "/profiles?page=0",
			want: map[string]any{
				"location": "query", "field": "page", "path": "page", "rule": "minimum", "message": "页码不能小于 1",
			},
		},
		{
			name:   "path minimum",
			method: http.MethodGet,
			target: "/profiles/0",
			want: map[string]any{
				"location": "path", "field": "profileId", "path": "profileId", "rule": "minimum", "message": "profileId 必须是正整数",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var body io.Reader
			if tt.body != "" {
				body = strings.NewReader(tt.body)
			}
			request := httptest.NewRequest(tt.method, tt.target, body)
			if tt.body != "" {
				request.Header.Set("Content-Type", "application/json")
			}
			recorder := httptest.NewRecorder()
			handler.ServeHTTP(recorder, request)

			responseBody := assertContractError(t, recorder, http.StatusBadRequest, validationEnvelopeMessage(tt.want["location"].(string)))
			assertSingleViolation(t, responseBody, tt.want)
		})
	}
}

func validationEnvelopeMessage(location string) string {
	if location == "body" {
		return "请求体参数校验失败"
	}
	return "请求参数校验失败"
}

func TestRequestValidatorUsesOperationValidationMessageAsFallback(t *testing.T) {
	handler := testRequestContractHandler(t, `openapi: 3.0.3
info:
  title: Operation Message Test API
  version: test
paths:
  /profiles:
    get:
      operationId: listProfiles
      x-validation:
        messages:
          minimum: operation minimum message
      parameters:
        - name: page
          in: query
          schema:
            type: integer
            minimum: 1
      responses:
        "204":
          description: no content
`)

	request := httptest.NewRequest(http.MethodGet, "/profiles?page=0", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	body := assertContractError(t, recorder, http.StatusBadRequest, "请求参数校验失败")
	assertSingleViolation(t, body, map[string]any{
		"location": "query",
		"field":    "page",
		"path":     "page",
		"rule":     "minimum",
		"message":  "operation minimum message",
	})
}

func TestRequestValidatorUsesCatalogMessageForAllOfBodyViolation(t *testing.T) {
	handler := testRequestContractHandler(t, `openapi: 3.0.3
info:
  title: AllOf Validation Catalog Test API
  version: test
paths:
  /profiles:
    post:
      operationId: createProfile
      requestBody:
        required: true
        content:
          application/json:
            schema:
              allOf:
                - type: object
                  required:
                    - name
                  properties:
                    name:
                      type: string
                      minLength: 2
                      x-validation:
                        messages:
                          required: 姓名不能为空
                          minLength: 姓名至少 2 个字符
      responses:
        "204":
          description: no content
`)

	request := httptest.NewRequest(http.MethodPost, "/profiles", strings.NewReader(`{"name":"a"}`))
	request.Header.Set("Content-Type", "application/json")
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

func TestNewRequestValidatorRejectsInvalidValidationExtensions(t *testing.T) {
	tests := []struct {
		name      string
		extension string
		wantError string
	}{
		{
			name:      "messages is not a string map",
			extension: "                    messages: []",
			wantError: "x-validation.messages must be a non-empty string map",
		},
		{
			name: "native rule message is missing",
			extension: `                    messages:
                      maximum: too long`,
			wantError: "native rule minLength must declare a non-empty messages.minLength",
		},
		{
			name: "custom rule is unknown",
			extension: `                    messages:
                      minLength: too short
                    rules:
                      - name: unknownRule
                        message: invalid`,
			wantError: `uses unsupported custom rule "unknownRule"`,
		},
		{
			name: "notBlank is false",
			extension: `                    notBlank: false
                    messages:
                      minLength: too short`,
			wantError: "x-validation.notBlank must be true when declared",
		},
		{
			name: "message rules collide after trimming",
			extension: `                    messages:
                      minLength: too short
                      ' minLength ': duplicate`,
			wantError: `duplicates rule "minLength" after trimming`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawSpec := fmt.Sprintf(`openapi: 3.0.3
info:
  title: Invalid Validation Extension Test API
  version: test
paths:
  /profiles:
    post:
      operationId: createProfile
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                  minLength: 2
                  x-validation:
%s
      responses:
        "204":
          description: no content
`, tt.extension)
			spec, err := LoadSpec(writeSpec(t, rawSpec))
			if err != nil {
				t.Fatalf("load spec before catalog compilation: %v", err)
			}
			_, err = NewRequestValidator(spec, WithRequestValidation(false))
			if err == nil || !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("NewRequestValidator() error = %v, want substring %q", err, tt.wantError)
			}
		})
	}
}

func TestNewRequestValidatorRejectsInvalidOperationValidationExtension(t *testing.T) {
	spec, err := LoadSpec(writeSpec(t, `openapi: 3.0.3
info:
  title: Invalid Operation Validation Extension Test API
  version: test
paths:
  /profiles:
    get:
      operationId: listProfiles
      x-validation:
        messages:
          minimum: operation minimum message
        crossFields:
          - name: createdAtRange
            rule: unknownRule
            left: createdAtFrom
            right: createdAtTo
            message: invalid
      responses:
        "204":
          description: no content
`))
	if err != nil {
		t.Fatalf("load spec before catalog compilation: %v", err)
	}

	_, err = NewRequestValidator(spec)
	if err == nil || !strings.Contains(err.Error(), `uses unsupported custom rule "unknownRule"`) {
		t.Fatalf("NewRequestValidator() error = %v, want unsupported operation custom rule", err)
	}
}

func TestRequestValidatorMapsHeaderParameterViolation(t *testing.T) {
	handler := testRequestContractHandler(t, `openapi: 3.0.3
info:
  title: Header Contract Test API
  version: test
paths:
  /tenants:
    get:
      operationId: listTenants
      parameters:
        - name: X-Tenant-Id
          in: header
          required: true
          schema:
            type: integer
            minimum: 1
      responses:
        "204":
          description: no content
`)

	request := httptest.NewRequest(http.MethodGet, "/tenants", nil)
	request.Header.Set("X-Tenant-Id", "0")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	body := assertContractError(t, recorder, http.StatusBadRequest, "请求头参数校验失败")
	assertSingleViolation(t, body, map[string]any{
		"location": "header",
		"field":    "X-Tenant-Id",
		"path":     "X-Tenant-Id",
		"rule":     "minimum",
		"message":  "X-Tenant-Id 不符合请求头契约",
	})
}

func TestRequestValidatorMapsCookieParameterViolation(t *testing.T) {
	handler := testRequestContractHandler(t, `openapi: 3.0.3
info:
  title: Cookie Contract Test API
  version: test
paths:
  /sessions:
    get:
      operationId: getSession
      parameters:
        - name: session
          in: cookie
          required: true
          schema:
            type: string
            minLength: 8
      responses:
        "204":
          description: no content
`)

	request := httptest.NewRequest(http.MethodGet, "/sessions", nil)
	request.AddCookie(&http.Cookie{Name: "session", Value: "short"})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	body := assertContractError(t, recorder, http.StatusBadRequest, "Cookie 参数校验失败")
	assertSingleViolation(t, body, map[string]any{
		"location": "cookie",
		"field":    "session",
		"path":     "session",
		"rule":     "minLength",
		"message":  "session 不符合 Cookie 契约",
	})
}

func TestRequestValidatorMapsDisallowedEmptyParameterViolation(t *testing.T) {
	handler := testRequestContractHandler(t, `openapi: 3.0.3
info:
  title: Empty Parameter Contract Test API
  version: test
paths:
  /search:
    get:
      operationId: search
      parameters:
        - name: filter
          in: query
          required: true
          allowEmptyValue: false
          schema:
            type: string
      responses:
        "204":
          description: no content
`)

	request := httptest.NewRequest(http.MethodGet, "/search?filter=", nil)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	body := assertContractError(t, recorder, http.StatusBadRequest, "请求参数校验失败")
	assertSingleViolation(t, body, map[string]any{
		"location": "query",
		"field":    "filter",
		"path":     "filter",
		"rule":     "allowEmptyValue",
		"message":  "filter 不符合查询参数契约",
	})
}

func TestRequestValidatorAllowsUnknownQueryHeaderAndCookieParameters(t *testing.T) {
	handler := testRequestContractHandler(t, `openapi: 3.0.3
info:
  title: Unknown Parameter Contract Test API
  version: test
paths:
  /ping:
    get:
      operationId: ping
      responses:
        "204":
          description: no content
`)

	request := httptest.NewRequest(http.MethodGet, "/ping?unknown=query", nil)
	request.Header.Set("X-Unknown", "header")
	request.AddCookie(&http.Cookie{Name: "unknown", Value: "cookie"})
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("expected unknown query/header/cookie to pass for the current policy, got status %d body=%s", recorder.Code, recorder.Body.String())
	}
}

const validationCatalogRuntimeSpec = `openapi: 3.0.3
info:
  title: Validation Catalog Runtime Test API
  version: test
paths:
  /profiles:
    get:
      operationId: listProfiles
      x-validation:
        crossFields:
          - name: createdAtRange
            rule: notAfter
            left: createdAtFrom
            right: createdAtTo
            message: createdAtFrom 不能晚于 createdAtTo
      parameters:
        - name: page
          in: query
          schema:
            type: integer
            minimum: 1
            maximum: 100
            x-validation:
              messages:
                minimum: 页码不能小于 1
                maximum: 页码不能大于 100
        - name: createdAtFrom
          in: query
          schema:
            type: string
            pattern: '^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}$'
            x-validation:
              messages:
                pattern: createdAtFrom 格式不合法
        - name: createdAtTo
          in: query
          schema:
            type: string
            pattern: '^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}$'
            x-validation:
              messages:
                pattern: createdAtTo 格式不合法
      responses:
        "204":
          description: no content
    post:
      operationId: createProfile
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required:
                - name
              properties:
                name:
                  type: string
                  minLength: 2
                  maxLength: 5
                  x-validation:
                    notBlank: true
                    messages:
                      required: 姓名不能为空
                      notBlank: 姓名不能为空
                      minLength: 姓名至少 2 个字符
                      maxLength: 姓名不能超过 5 个字符
                password:
                  type: string
                  minLength: 8
                  x-validation:
                    messages:
                      minLength: password 至少 8 个字符
                    rules:
                      - name: containsLetterAndDigit
                        message: password 至少包含字母和数字
                code:
                  type: string
                  pattern: '^[A-Z]+$'
                  x-validation:
                    messages:
                      pattern: code 只能使用大写字母
                createdAt:
                  type: string
                  format: date-time
                  x-validation:
                    messages:
                      format: createdAt 格式不合法
                status:
                  type: string
                  enum:
                    - ENABLED
                    - DISABLED
                  x-validation:
                    messages:
                      enum: status 只能是 ENABLED 或 DISABLED
                score:
                  type: integer
                  minimum: 1
                  maximum: 10
                  x-validation:
                    messages:
                      minimum: score 不能小于 1
                      maximum: score 不能大于 10
      responses:
        "204":
          description: no content
  /profiles/{profileId}:
    get:
      operationId: getProfile
      parameters:
        - name: profileId
          in: path
          required: true
          schema:
            type: integer
            minimum: 1
            x-validation:
              messages:
                required: profileId 不能为空
                minimum: profileId 必须是正整数
      responses:
        "204":
          description: no content
`

func testRequestContractHandler(t *testing.T, rawSpec string) http.Handler {
	t.Helper()

	spec, err := LoadSpec(writeSpec(t, rawSpec))
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	validator, err := NewRequestValidator(spec)
	if err != nil {
		t.Fatalf("new request validator: %v", err)
	}
	return validator.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
}

func assertContractError(t *testing.T, recorder *httptest.ResponseRecorder, status int, message string) map[string]any {
	t.Helper()

	if recorder.Code != status {
		t.Fatalf("unexpected status: %d body=%s", recorder.Code, recorder.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response body: %v body=%s", err, recorder.Body.String())
	}
	if body["code"] != "COMMON-400" {
		t.Fatalf("unexpected code: %v", body["code"])
	}
	if body["message"] != message {
		t.Fatalf("unexpected message: %v", body["message"])
	}
	data, ok := body["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %#v", body["data"])
	}
	violations, ok := data["violations"].([]any)
	if !ok || len(violations) == 0 {
		t.Fatalf("expected non-empty data.violations, got %#v", data)
	}
	for _, item := range violations {
		violation, ok := item.(map[string]any)
		if !ok || len(violation) != 5 {
			t.Fatalf("expected five-field violation, got %#v", item)
		}
		for _, field := range []string{"location", "field", "path", "rule", "message"} {
			if _, ok := violation[field].(string); !ok {
				t.Fatalf("violation.%s must be a string: %#v", field, violation)
			}
		}
	}
	return body
}

func assertSingleViolation(t *testing.T, body map[string]any, want map[string]any) {
	t.Helper()
	data := body["data"].(map[string]any)
	violations := data["violations"].([]any)
	if len(violations) != 1 {
		t.Fatalf("violations count = %d, want 1: %#v", len(violations), violations)
	}
	got := violations[0].(map[string]any)
	for field, wantValue := range want {
		if got[field] != wantValue {
			t.Fatalf("violation.%s = %#v, want %#v: %#v", field, got[field], wantValue, got)
		}
	}
}
