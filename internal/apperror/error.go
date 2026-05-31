package apperror

import (
	"errors"
	"net/http"
)

type Code struct {
	value          string
	httpStatus     int
	defaultMessage string
}

var (
	CommonSuccess    = Code{value: "COMMON-000", httpStatus: http.StatusOK, defaultMessage: "成功"}
	CommonValidation = Code{value: "COMMON-400", httpStatus: http.StatusBadRequest, defaultMessage: "请求参数不合法"}
	CommonBusiness   = Code{value: "COMMON-401", httpStatus: http.StatusBadRequest, defaultMessage: "业务处理失败"}
	CommonNotFound   = Code{value: "COMMON-404", httpStatus: http.StatusNotFound, defaultMessage: "资源不存在"}
	CommonInternal   = Code{value: "COMMON-500", httpStatus: http.StatusInternalServerError, defaultMessage: "系统内部错误"}
	AuthUnauthorized = Code{value: "AUTH-401", httpStatus: http.StatusUnauthorized, defaultMessage: "认证失败"}
	AuthForbidden    = Code{value: "AUTH-403", httpStatus: http.StatusForbidden, defaultMessage: "权限不足"}
	AuthConflict     = Code{value: "AUTH-409", httpStatus: http.StatusConflict, defaultMessage: "账号信息已存在"}
)

func (c Code) String() string {
	return c.value
}

func (c Code) HTTPStatus() int {
	if c.httpStatus == 0 {
		return http.StatusInternalServerError
	}
	return c.httpStatus
}

func (c Code) DefaultMessage() string {
	return c.defaultMessage
}

type AppError struct {
	code    Code
	message string
	data    any
	cause   error
}

func New(code Code, message string) *AppError {
	code = normalizeCode(code)
	if message == "" {
		message = code.DefaultMessage()
	}
	return &AppError{code: code, message: message}
}

func WithData(code Code, message string, data any) *AppError {
	err := New(code, message)
	err.data = data
	return err
}

func Wrap(code Code, message string, cause error) *AppError {
	err := New(code, message)
	err.cause = cause
	return err
}

func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	return e.message
}

func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

func (e *AppError) Code() Code {
	if e == nil {
		return CommonInternal
	}
	return e.code
}

func (e *AppError) Message() string {
	if e == nil {
		return CommonInternal.DefaultMessage()
	}
	return e.message
}

func (e *AppError) Data() any {
	if e == nil {
		return nil
	}
	return e.data
}

func FromError(err error) (*AppError, bool) {
	if err == nil {
		return nil, false
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}

func normalizeCode(code Code) Code {
	if code.value == "" {
		return CommonInternal
	}
	return code
}
