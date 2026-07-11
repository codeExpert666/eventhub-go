package contract

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

const emailFormatRuntimeSpec = `openapi: 3.0.3
info:
  title: Email Format Runtime Test API
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
              required:
                - email
              properties:
                email:
                  type: string
                  format: email
                  x-validation:
                    messages:
                      required: email 不能为空
                      format: email 格式不合法
      responses:
        "204":
          description: no content
`

func TestRequestValidatorExecutesDeclaredEmailFormat(t *testing.T) {
	handler := testRequestContractHandler(t, emailFormatRuntimeSpec)
	for _, value := range []string{
		"not-an-email",
		"foo bar@example.com",
		".foo@example.com",
		"foo..bar@example.com",
		strings.Repeat("a", maxEmailLocalPartLength+1) + "@example.com",
		strings.Repeat("😀", maxEmailLocalPartLength/2+1) + "@example.com",
		"Alice <alice@example.com>",
	} {
		t.Run(value, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodPost,
				"/profiles",
				strings.NewReader(`{"email":`+strconv.Quote(value)+`}`),
			)
			request.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			handler.ServeHTTP(recorder, request)

			body := assertContractError(t, recorder, http.StatusBadRequest, "请求体参数校验失败")
			assertSingleViolation(t, body, map[string]any{
				"location": "body",
				"field":    "email",
				"path":     "email",
				"rule":     "format",
				"message":  "email 格式不合法",
			})
		})
	}
}

func TestRequestValidatorAllowsValidDeclaredEmailFormat(t *testing.T) {
	handler := testRequestContractHandler(t, emailFormatRuntimeSpec)
	request := httptest.NewRequest(http.MethodPost, "/profiles", strings.NewReader(`{"email":"alice@example.com"}`))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusNoContent, recorder.Body.String())
	}
}
