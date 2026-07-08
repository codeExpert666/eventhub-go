package requesterror

import "eventhub-go/internal/apperror"

// FieldErrors 表示按字段名归集的请求参数校验错误。
type FieldErrors = apperror.Details

// InvalidBody 根据字段校验结果构造统一的请求体参数校验错误。
func InvalidBody(fields FieldErrors) *apperror.AppError {
	return apperror.WithDetails(
		apperror.CommonValidation,
		"请求体参数校验失败",
		fields,
	)
}

// MalformedBody 构造请求体缺失、格式错误或包含非法内容时的统一错误。
func MalformedBody() *apperror.AppError {
	return apperror.WithDetails(
		apperror.CommonValidation,
		"请求体格式不合法",
		FieldErrors{"body": "请求体缺失或 JSON 格式错误"},
	)
}

// InvalidParameters 根据字段校验结果构造统一的 path/query 参数校验错误。
func InvalidParameters(fields FieldErrors) *apperror.AppError {
	return apperror.WithDetails(
		apperror.CommonValidation,
		"请求参数校验失败",
		fields,
	)
}

// InvalidHeaders 根据字段校验结果构造统一的请求头参数校验错误。
func InvalidHeaders(fields FieldErrors) *apperror.AppError {
	return apperror.WithDetails(
		apperror.CommonValidation,
		"请求头参数校验失败",
		fields,
	)
}

// InvalidCookies 根据字段校验结果构造统一的 Cookie 参数校验错误。
func InvalidCookies(fields FieldErrors) *apperror.AppError {
	return apperror.WithDetails(
		apperror.CommonValidation,
		"Cookie 参数校验失败",
		fields,
	)
}

// UnsupportedContentType 构造请求体 Content-Type 不符合 OpenAPI 契约时的统一错误。
func UnsupportedContentType(contentType string) *apperror.AppError {
	if contentType == "" {
		contentType = "缺少 Content-Type"
	}
	return apperror.WithDetails(
		apperror.CommonValidation,
		"请求内容类型不支持",
		FieldErrors{"Content-Type": contentType},
	)
}
