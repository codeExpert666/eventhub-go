package requesterror

import "eventhub-go/internal/apperror"

const (
	LocationBody   = "body"
	LocationQuery  = "query"
	LocationPath   = "path"
	LocationHeader = "header"
	LocationCookie = "cookie"
)

// Violation 表示一条稳定、可定位的 HTTP 请求契约错误。
type Violation struct {
	Location string `json:"location"`
	Field    string `json:"field"`
	Path     string `json:"path"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
}

// Violations 表示同一次 HTTP 请求中的字段契约错误列表。
type Violations []Violation

// InvalidBody 根据字段校验结果构造统一的请求体参数校验错误。
func InvalidBody(violations Violations) *apperror.AppError {
	return apperror.WithDetails(
		apperror.CommonValidation,
		"请求体参数校验失败",
		violationDetails(violations),
	)
}

// MissingBody 构造必填请求体缺失时的统一错误。
func MissingBody() *apperror.AppError {
	return invalidBodyFormat("required")
}

// MalformedBody 构造请求体格式错误或包含非法内容时的统一错误。
func MalformedBody() *apperror.AppError {
	return invalidBodyFormat("malformed")
}

func invalidBodyFormat(rule string) *apperror.AppError {
	return apperror.WithDetails(
		apperror.CommonValidation,
		"请求体格式不合法",
		violationDetails(Violations{{
			Location: LocationBody,
			Field:    "body",
			Path:     "body",
			Rule:     rule,
			Message:  "请求体缺失或 JSON 格式错误",
		}}),
	)
}

// InvalidParameters 根据字段校验结果构造统一的 path/query 参数校验错误。
func InvalidParameters(violations Violations) *apperror.AppError {
	return apperror.WithDetails(
		apperror.CommonValidation,
		"请求参数校验失败",
		violationDetails(violations),
	)
}

// InvalidHeaders 根据字段校验结果构造统一的请求头参数校验错误。
func InvalidHeaders(violations Violations) *apperror.AppError {
	return apperror.WithDetails(
		apperror.CommonValidation,
		"请求头参数校验失败",
		violationDetails(violations),
	)
}

// InvalidCookies 根据字段校验结果构造统一的 Cookie 参数校验错误。
func InvalidCookies(violations Violations) *apperror.AppError {
	return apperror.WithDetails(
		apperror.CommonValidation,
		"Cookie 参数校验失败",
		violationDetails(violations),
	)
}

// UnsupportedContentType 构造请求体 Content-Type 不符合 OpenAPI 契约时的统一错误。
func UnsupportedContentType(contentType string) *apperror.AppError {
	message := "缺少 Content-Type"
	if contentType != "" {
		message = "不支持的 Content-Type: " + contentType
	}
	return apperror.WithDetails(
		apperror.CommonValidation,
		"请求内容类型不支持",
		violationDetails(Violations{{
			Location: LocationHeader,
			Field:    "Content-Type",
			Path:     "Content-Type",
			Rule:     "contentType",
			Message:  message,
		}}),
	)
}

func violationDetails(violations Violations) apperror.Details {
	if violations == nil {
		violations = Violations{}
	}
	return apperror.Details{"violations": violations}
}
