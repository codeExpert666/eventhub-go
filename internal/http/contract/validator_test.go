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
	data := body["data"].(map[string]any)
	if data["X-Tenant-Id"] != "X-Tenant-Id 不符合请求头契约" {
		t.Fatalf("unexpected header violation: %#v", data)
	}
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
	data := body["data"].(map[string]any)
	if data["session"] != "session 不符合 Cookie 契约" {
		t.Fatalf("unexpected cookie violation: %#v", data)
	}
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
	if _, ok := body["data"].(map[string]any); !ok {
		t.Fatalf("expected data object, got %#v", body["data"])
	}
	return body
}
