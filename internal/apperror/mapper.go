package apperror

import "errors"

// FromError 从错误链中提取 AppError。
func FromError(err error) (*AppError, bool) {
	if err == nil {
		return nil, false
	}
	var appErr *AppError
	if errors.As(err, &appErr) && appErr != nil {
		return appErr, true
	}
	return nil, false
}

// FromErrorOrInternal 将任意错误收敛为 AppError。
// 已经是 AppError 的错误会原样返回；其他错误按系统内部错误包装。
func FromErrorOrInternal(err error) *AppError {
	if appErr, ok := FromError(err); ok {
		return appErr
	}
	return Wrap(CommonInternal, "", err)
}

// normalizeCode 将空错误码规范化为通用内部错误码。
func normalizeCode(code Code) Code {
	if code.value == "" {
		return CommonInternal
	}
	return code
}
