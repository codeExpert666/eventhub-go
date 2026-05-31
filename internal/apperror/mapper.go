package apperror

import "errors"

// FromError 从错误链中提取 AppError。
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

// normalizeCode 将空错误码规范化为通用内部错误码。
func normalizeCode(code Code) Code {
	if code.value == "" {
		return CommonInternal
	}
	return code
}
