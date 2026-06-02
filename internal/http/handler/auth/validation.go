package auth

import (
	"net/mail"
	"regexp"
	"strings"

	"eventhub-go/internal/apperror"
	authdto "eventhub-go/internal/http/dto/auth"
	"eventhub-go/internal/http/validation"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func validateRegisterRequest(request authdto.RegisterRequest) *apperror.AppError {
	fields := validation.FieldErrors{}
	username := strings.TrimSpace(request.Username)
	email := strings.TrimSpace(request.Email)
	password := request.Password

	if username == "" {
		fields["username"] = "username 不能为空"
	} else if len(username) < 3 || len(username) > 32 {
		fields["username"] = "username 长度必须在 3 到 32 个字符之间"
	} else if !usernamePattern.MatchString(username) {
		fields["username"] = "username 只能包含字母、数字和下划线"
	}

	if email == "" {
		fields["email"] = "email 不能为空"
	} else if len(email) > 128 {
		fields["email"] = "email 长度不能超过 128 个字符"
	} else if _, err := mail.ParseAddress(email); err != nil {
		fields["email"] = "email 格式不合法"
	}

	if password == "" {
		fields["password"] = "password 不能为空"
	} else if len(password) < 8 || len(password) > 72 {
		fields["password"] = "password 长度必须在 8 到 72 个字符之间"
	} else if !containsLetterAndDigit(password) {
		fields["password"] = "password 至少包含字母和数字"
	}

	if len(fields) > 0 {
		return validation.BodyValidationError(fields)
	}
	return nil
}

func validateLoginRequest(request authdto.LoginRequest) *apperror.AppError {
	fields := validation.FieldErrors{}
	usernameOrEmail := strings.TrimSpace(request.UsernameOrEmail)

	if usernameOrEmail == "" {
		fields["usernameOrEmail"] = "用户名或邮箱不能为空"
	} else if len(usernameOrEmail) > 128 {
		fields["usernameOrEmail"] = "用户名或邮箱长度不能超过 128 个字符"
	}
	if request.Password == "" {
		fields["password"] = "密码不能为空"
	} else if len(request.Password) > 72 {
		fields["password"] = "密码长度不能超过 72 个字符"
	}

	if len(fields) > 0 {
		return validation.BodyValidationError(fields)
	}
	return nil
}

func containsLetterAndDigit(value string) bool {
	hasLetter := false
	hasDigit := false
	for _, char := range value {
		if (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') {
			hasLetter = true
		}
		if char >= '0' && char <= '9' {
			hasDigit = true
		}
	}
	return hasLetter && hasDigit
}
