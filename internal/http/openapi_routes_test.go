package http

import (
	"errors"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	openapigen "eventhub-go/api/openapi/gen"
	"eventhub-go/internal/http/requesterror"
)

func TestParameterValidationErrorMapsRequiredQueryParameter(t *testing.T) {
	request := httptest.NewRequest(nethttp.MethodGet, "/search", nil)

	appErr := parameterValidationError(request, &openapigen.RequiredParamError{ParamName: "filter"})

	if appErr.Message() != "请求参数校验失败" {
		t.Fatalf("message = %q, want 请求参数校验失败", appErr.Message())
	}
	assertRequestViolation(t, appErr.Details()["violations"], requesterror.Violation{
		Location: requesterror.LocationQuery,
		Field:    "filter",
		Path:     "filter",
		Rule:     "required",
		Message:  "filter 不能为空",
	})
}

func TestParameterValidationErrorMapsRequiredHeader(t *testing.T) {
	request := httptest.NewRequest(nethttp.MethodGet, "/search", nil)

	appErr := parameterValidationError(request, &openapigen.RequiredHeaderError{
		ParamName: "X-Tenant-Id",
		Err:       errors.New("missing header"),
	})

	if appErr.Message() != "请求头参数校验失败" {
		t.Fatalf("message = %q, want 请求头参数校验失败", appErr.Message())
	}
	assertRequestViolation(t, appErr.Details()["violations"], requesterror.Violation{
		Location: requesterror.LocationHeader,
		Field:    "X-Tenant-Id",
		Path:     "X-Tenant-Id",
		Rule:     "required",
		Message:  "X-Tenant-Id 不能为空",
	})
}

func assertRequestViolation(t *testing.T, raw any, want requesterror.Violation) {
	t.Helper()
	violations, ok := raw.(requesterror.Violations)
	if !ok || len(violations) != 1 {
		t.Fatalf("violations = %#v, want one requesterror.Violation", raw)
	}
	if violations[0] != want {
		t.Fatalf("violation = %#v, want %#v", violations[0], want)
	}
}
