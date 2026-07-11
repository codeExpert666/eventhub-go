package requesterror_test

import (
	"reflect"
	"testing"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/requesterror"
)

func TestInvalidBody(t *testing.T) {
	violation := requesterror.Violation{
		Location: requesterror.LocationBody,
		Field:    "username",
		Path:     "username",
		Rule:     "notBlank",
		Message:  "username 不能为空",
	}

	err := requesterror.InvalidBody(requesterror.Violations{violation})

	assertValidationError(t, err, "请求体参数校验失败", violation)
}

func TestMalformedBody(t *testing.T) {
	err := requesterror.MalformedBody()

	assertValidationError(t, err, "请求体格式不合法", requesterror.Violation{
		Location: requesterror.LocationBody,
		Field:    "body",
		Path:     "body",
		Rule:     "malformed",
		Message:  "请求体缺失或 JSON 格式错误",
	})
}

func TestMissingBody(t *testing.T) {
	err := requesterror.MissingBody()

	assertValidationError(t, err, "请求体格式不合法", requesterror.Violation{
		Location: requesterror.LocationBody,
		Field:    "body",
		Path:     "body",
		Rule:     "required",
		Message:  "请求体缺失或 JSON 格式错误",
	})
}

func TestInvalidParameters(t *testing.T) {
	violation := requesterror.Violation{
		Location: requesterror.LocationQuery,
		Field:    "page",
		Path:     "page",
		Rule:     "type",
		Message:  "page 必须是整数",
	}

	err := requesterror.InvalidParameters(requesterror.Violations{violation})

	assertValidationError(t, err, "请求参数校验失败", violation)
}

func TestInvalidHeaders(t *testing.T) {
	violation := requesterror.Violation{
		Location: requesterror.LocationHeader,
		Field:    "X-Tenant-Id",
		Path:     "X-Tenant-Id",
		Rule:     "minimum",
		Message:  "X-Tenant-Id 不符合请求头契约",
	}

	err := requesterror.InvalidHeaders(requesterror.Violations{violation})

	assertValidationError(t, err, "请求头参数校验失败", violation)
}

func TestInvalidCookies(t *testing.T) {
	violation := requesterror.Violation{
		Location: requesterror.LocationCookie,
		Field:    "session",
		Path:     "session",
		Rule:     "minLength",
		Message:  "session 不符合 Cookie 契约",
	}

	err := requesterror.InvalidCookies(requesterror.Violations{violation})

	assertValidationError(t, err, "Cookie 参数校验失败", violation)
}

func TestUnsupportedContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		message     string
	}{
		{name: "unsupported", contentType: "text/plain", message: "不支持的 Content-Type: text/plain"},
		{name: "missing", message: "缺少 Content-Type"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := requesterror.UnsupportedContentType(tt.contentType)

			assertValidationError(t, err, "请求内容类型不支持", requesterror.Violation{
				Location: requesterror.LocationHeader,
				Field:    "Content-Type",
				Path:     "Content-Type",
				Rule:     "contentType",
				Message:  tt.message,
			})
		})
	}
}

func assertValidationError(t *testing.T, err *apperror.AppError, message string, want requesterror.Violation) {
	t.Helper()
	if err.Code() != apperror.CommonValidation {
		t.Fatalf("unexpected code: %s", err.Code().String())
	}
	if err.Message() != message {
		t.Fatalf("unexpected message: %s", err.Message())
	}
	details := err.Details()
	if len(details) != 1 {
		t.Fatalf("unexpected details keys: %#v", details)
	}
	got, ok := details["violations"].(requesterror.Violations)
	if !ok {
		t.Fatalf("violations type = %T, want requesterror.Violations", details["violations"])
	}
	if !reflect.DeepEqual(got, requesterror.Violations{want}) {
		t.Fatalf("violations = %#v, want %#v", got, requesterror.Violations{want})
	}
}
