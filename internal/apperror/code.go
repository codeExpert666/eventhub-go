package apperror

import "net/http"

// Code 表示可对外稳定暴露的应用错误码，并携带默认 HTTP 状态码和提示消息。
type Code struct {
	value          string
	httpStatus     int
	defaultMessage string
}

var (
	// CommonSuccess 表示通用成功结果。
	CommonSuccess = Code{value: "COMMON-000", httpStatus: http.StatusOK, defaultMessage: "成功"}
	// CommonValidation 表示通用请求参数校验失败。
	CommonValidation = Code{value: "COMMON-400", httpStatus: http.StatusBadRequest, defaultMessage: "请求参数不合法"}
	// CommonBusiness 表示通用业务处理失败。
	CommonBusiness = Code{value: "COMMON-401", httpStatus: http.StatusBadRequest, defaultMessage: "业务处理失败"}
	// CommonNotFound 表示请求的业务资源不存在。
	CommonNotFound = Code{value: "COMMON-404", httpStatus: http.StatusNotFound, defaultMessage: "资源不存在"}
	// CommonInternal 表示服务端未预期的内部错误。
	CommonInternal = Code{value: "COMMON-500", httpStatus: http.StatusInternalServerError, defaultMessage: "系统内部错误"}
	// AuthUnauthorized 表示认证失败或认证信息无效。
	AuthUnauthorized = Code{value: "AUTH-401", httpStatus: http.StatusUnauthorized, defaultMessage: "认证失败"}
	// AuthForbidden 表示当前身份缺少访问目标资源所需权限。
	AuthForbidden = Code{value: "AUTH-403", httpStatus: http.StatusForbidden, defaultMessage: "权限不足"}
	// AuthConflict 表示账号、身份凭据等认证相关资源发生唯一性冲突。
	AuthConflict = Code{value: "AUTH-409", httpStatus: http.StatusConflict, defaultMessage: "账号信息已存在"}
)

// String 返回对外暴露的稳定错误码字符串。
func (c Code) String() string {
	return c.value
}

// HTTPStatus 返回错误码映射的 HTTP 状态码，未配置时退化为 500。
func (c Code) HTTPStatus() int {
	if c.httpStatus == 0 {
		return http.StatusInternalServerError
	}
	return c.httpStatus
}

// DefaultMessage 返回错误码对应的默认用户可读提示。
func (c Code) DefaultMessage() string {
	return c.defaultMessage
}
