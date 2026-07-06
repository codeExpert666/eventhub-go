package validation

import "eventhub-go/internal/apperror"

// FieldErrors 表示按字段名归集的请求参数校验错误。
type FieldErrors = apperror.Details

// BodyValidationError 根据字段校验结果构造统一的请求体参数校验错误。
func BodyValidationError(fields FieldErrors) *apperror.AppError {
	return apperror.WithDetails(
		apperror.CommonValidation,
		"请求体参数校验失败",
		fields,
	)
}

// MalformedBodyError 构造请求体缺失、格式错误或包含非法内容时的统一错误。
func MalformedBodyError() *apperror.AppError {
	return apperror.WithDetails(
		apperror.CommonValidation,
		"请求体格式不合法",
		FieldErrors{"body": "请求体缺失或 JSON 格式错误"},
	)
}

// ParameterValidationError 根据字段校验结果构造统一的 path/query 参数校验错误。
func ParameterValidationError(fields FieldErrors) *apperror.AppError {
	return apperror.WithDetails(
		apperror.CommonValidation,
		"请求参数校验失败",
		fields,
	)
}
