package validation_test

import (
	"testing"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/validation"
)

func TestBodyValidationError(t *testing.T) {
	err := validation.BodyValidationError(validation.FieldErrors{
		"username": "username 不能为空",
	})

	assertValidationError(t, err, "请求体参数校验失败", "username", "username 不能为空")
}

func TestMalformedBodyError(t *testing.T) {
	err := validation.MalformedBodyError()

	assertValidationError(t, err, "请求体格式不合法", "body", "请求体缺失或 JSON 格式错误")
}

func TestParameterValidationError(t *testing.T) {
	err := validation.ParameterValidationError(validation.FieldErrors{
		"page": "page 必须是整数",
	})

	assertValidationError(t, err, "请求参数校验失败", "page", "page 必须是整数")
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
