package validation

import "eventhub-go/internal/apperror"

// AppErrorFromError 将任意 error 映射为可写出的 AppError。
func AppErrorFromError(err error) *apperror.AppError {
	if appErr, ok := apperror.FromError(err); ok {
		return appErr
	}
	return apperror.Wrap(apperror.CommonInternal, "", err)
}
