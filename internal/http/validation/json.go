package validation

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"eventhub-go/internal/apperror"
)

type FieldErrors map[string]string

func DecodeJSONBody(r *http.Request, dst any) *apperror.AppError {
	if r.Body == nil {
		return malformedBodyError()
	}

	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(dst); err != nil {
		return decodeError(err)
	}

	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return malformedBodyError()
	}
	return nil
}

func BodyValidationError(fields FieldErrors) *apperror.AppError {
	return apperror.WithData(
		apperror.CommonValidation,
		"请求体参数校验失败",
		fields,
	)
}

func malformedBodyError() *apperror.AppError {
	return apperror.WithData(
		apperror.CommonValidation,
		"请求体格式不合法",
		map[string]string{"body": "请求体缺失或 JSON 格式错误"},
	)
}

func decodeError(err error) *apperror.AppError {
	var syntaxError *json.SyntaxError
	var typeError *json.UnmarshalTypeError
	if errors.Is(err, io.EOF) || errors.As(err, &syntaxError) || errors.As(err, &typeError) {
		return malformedBodyError()
	}
	return malformedBodyError()
}
