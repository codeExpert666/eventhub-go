package apperror_test

import (
	"errors"
	"net/http"
	"testing"

	"eventhub-go/internal/apperror"
)

func TestCodeMapping(t *testing.T) {
	tests := []struct {
		name       string
		code       apperror.Code
		value      string
		statusCode int
		message    string
	}{
		{name: "success", code: apperror.CommonSuccess, value: "COMMON-000", statusCode: http.StatusOK, message: "成功"},
		{name: "validation", code: apperror.CommonValidation, value: "COMMON-400", statusCode: http.StatusBadRequest, message: "请求参数不合法"},
		{name: "business", code: apperror.CommonBusiness, value: "COMMON-401", statusCode: http.StatusBadRequest, message: "业务处理失败"},
		{name: "not found", code: apperror.CommonNotFound, value: "COMMON-404", statusCode: http.StatusNotFound, message: "资源不存在"},
		{name: "internal", code: apperror.CommonInternal, value: "COMMON-500", statusCode: http.StatusInternalServerError, message: "系统内部错误"},
		{name: "auth unauthorized", code: apperror.AuthUnauthorized, value: "AUTH-401", statusCode: http.StatusUnauthorized, message: "认证失败"},
		{name: "auth forbidden", code: apperror.AuthForbidden, value: "AUTH-403", statusCode: http.StatusForbidden, message: "权限不足"},
		{name: "auth conflict", code: apperror.AuthConflict, value: "AUTH-409", statusCode: http.StatusConflict, message: "账号信息已存在"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.code.String() != tt.value {
				t.Fatalf("code value mismatch: got %q want %q", tt.code.String(), tt.value)
			}
			if tt.code.HTTPStatus() != tt.statusCode {
				t.Fatalf("status mismatch: got %d want %d", tt.code.HTTPStatus(), tt.statusCode)
			}
			if tt.code.DefaultMessage() != tt.message {
				t.Fatalf("message mismatch: got %q want %q", tt.code.DefaultMessage(), tt.message)
			}
		})
	}
}

func TestAppError(t *testing.T) {
	err := apperror.New(apperror.CommonBusiness, "")
	if err.Code().String() != "COMMON-401" {
		t.Fatalf("unexpected code: %s", err.Code().String())
	}
	if err.Message() != "业务处理失败" {
		t.Fatalf("unexpected default message: %s", err.Message())
	}

	custom := apperror.WithDetails(apperror.CommonValidation, "请求体参数校验失败", apperror.Details{
		"message": "message 不能为空",
	})
	if custom.Message() != "请求体参数校验失败" {
		t.Fatalf("unexpected custom message: %s", custom.Message())
	}
	if custom.Details()["message"] != "message 不能为空" {
		t.Fatalf("unexpected validation details: %#v", custom.Details())
	}
}

func TestFromError(t *testing.T) {
	base := errors.New("db unavailable")
	wrapped := apperror.Wrap(apperror.CommonInternal, "", base)

	appErr, ok := apperror.FromError(wrapped)
	if !ok {
		t.Fatal("expected app error")
	}
	if appErr.Code().String() != "COMMON-500" {
		t.Fatalf("unexpected code: %s", appErr.Code().String())
	}
	if !errors.Is(wrapped, base) {
		t.Fatal("expected wrapped cause")
	}
}

func TestFromErrorOrInternal(t *testing.T) {
	t.Run("returns existing app error", func(t *testing.T) {
		appErr := apperror.New(apperror.CommonValidation, "请求参数校验失败")

		result := apperror.FromErrorOrInternal(appErr)

		if result != appErr {
			t.Fatal("expected existing app error to be returned")
		}
		if result.Code() != apperror.CommonValidation {
			t.Fatalf("unexpected code: %s", result.Code().String())
		}
	})

	t.Run("wraps plain error as internal", func(t *testing.T) {
		cause := errors.New("db unavailable")

		result := apperror.FromErrorOrInternal(cause)

		if result.Code() != apperror.CommonInternal {
			t.Fatalf("unexpected code: %s", result.Code().String())
		}
		if result.Message() != "系统内部错误" {
			t.Fatalf("unexpected message: %s", result.Message())
		}
		if !errors.Is(result, cause) {
			t.Fatal("expected wrapped cause")
		}
	})

	t.Run("wraps typed nil app error as internal", func(t *testing.T) {
		var appErr *apperror.AppError

		result := apperror.FromErrorOrInternal(appErr)

		if result == nil {
			t.Fatal("expected non-nil app error")
		}
		if result.Code() != apperror.CommonInternal {
			t.Fatalf("unexpected code: %s", result.Code().String())
		}
	})
}
