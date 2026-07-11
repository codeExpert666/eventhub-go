package contract

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
