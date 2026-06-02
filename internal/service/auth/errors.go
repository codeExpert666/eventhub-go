package auth

import "eventhub-go/internal/apperror"

func duplicateUsernameError() *apperror.AppError {
	return apperror.New(apperror.AuthConflict, "用户名已存在")
}

func duplicateEmailError() *apperror.AppError {
	return apperror.New(apperror.AuthConflict, "邮箱已存在")
}

func duplicateAccountError() *apperror.AppError {
	return apperror.New(apperror.AuthConflict, "用户名或邮箱已存在")
}

func badCredentialsError() *apperror.AppError {
	return apperror.New(apperror.AuthUnauthorized, "账号或密码错误")
}

func disabledUserError() *apperror.AppError {
	return apperror.New(apperror.AuthForbidden, "用户已被禁用")
}
