package requesterror_test

import (
	"testing"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/requesterror"
)

func TestInvalidBody(t *testing.T) {
	err := requesterror.InvalidBody(requesterror.FieldErrors{
		"username": "username 不能为空",
	})

	assertValidationError(t, err, "请求体参数校验失败", "username", "username 不能为空")
}

func TestMalformedBody(t *testing.T) {
	err := requesterror.MalformedBody()

	assertValidationError(t, err, "请求体格式不合法", "body", "请求体缺失或 JSON 格式错误")
}

func TestInvalidParameters(t *testing.T) {
	err := requesterror.InvalidParameters(requesterror.FieldErrors{
		"page": "page 必须是整数",
	})

	assertValidationError(t, err, "请求参数校验失败", "page", "page 必须是整数")
}

func TestInvalidHeaders(t *testing.T) {
	err := requesterror.InvalidHeaders(requesterror.FieldErrors{
		"X-Tenant-Id": "X-Tenant-Id 不符合请求头契约",
	})

	assertValidationError(t, err, "请求头参数校验失败", "X-Tenant-Id", "X-Tenant-Id 不符合请求头契约")
}

func TestInvalidCookies(t *testing.T) {
	err := requesterror.InvalidCookies(requesterror.FieldErrors{
		"session": "session 不符合 Cookie 契约",
	})

	assertValidationError(t, err, "Cookie 参数校验失败", "session", "session 不符合 Cookie 契约")
}

func assertValidationError(t *testing.T, err *apperror.AppError, message, field, fieldMessage string) {
	t.Helper()
	if err.Code() != apperror.CommonValidation {
		t.Fatalf("unexpected code: %s", err.Code().String())
	}
	if err.Message() != message {
		t.Fatalf("unexpected message: %s", err.Message())
	}
	if err.Details()[field] != fieldMessage {
		t.Fatalf("unexpected details: %#v", err.Details())
	}
}

func TestUnsupportedContentType(t *testing.T) {
	err := requesterror.UnsupportedContentType("text/plain")

	if err.Code() != apperror.CommonValidation {
		t.Fatalf("unexpected code: %s", err.Code().String())
	}
	if err.Message() != "请求内容类型不支持" {
		t.Fatalf("unexpected message: %s", err.Message())
	}
	if err.Details()["Content-Type"] != "text/plain" {
		t.Fatalf("unexpected details: %#v", err.Details())
	}
}
