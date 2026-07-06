package auth

import (
	"net/mail"
	"regexp"
	"strings"

	openapigen "eventhub-go/api/openapi/gen"
	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/validation"
	authsvc "eventhub-go/internal/service/auth"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func parseRegisterCommand(request *openapigen.RegisterRequest) (authsvc.RegisterCommand, *apperror.AppError) {
	if request == nil {
		return authsvc.RegisterCommand{}, validation.MalformedBodyError()
	}

	fields := validation.FieldErrors{}
	command := authsvc.RegisterCommand{
		Username: strings.TrimSpace(request.Username),
		Email:    strings.TrimSpace(string(request.Email)),
		Password: request.Password,
	}

	if command.Username == "" {
		fields["username"] = "username 不能为空"
	} else if len(command.Username) < 3 || len(command.Username) > 32 {
		fields["username"] = "username 长度必须在 3 到 32 个字符之间"
	} else if !usernamePattern.MatchString(command.Username) {
		fields["username"] = "username 只能包含字母、数字和下划线"
	}

	if command.Email == "" {
		fields["email"] = "email 不能为空"
	} else if len(command.Email) > 128 {
		fields["email"] = "email 长度不能超过 128 个字符"
	} else if _, err := mail.ParseAddress(command.Email); err != nil {
		fields["email"] = "email 格式不合法"
	}

	if command.Password == "" {
		fields["password"] = "password 不能为空"
	} else if len(command.Password) < 8 || len(command.Password) > 72 {
		fields["password"] = "password 长度必须在 8 到 72 个字符之间"
	} else if !containsLetterAndDigit(command.Password) {
		fields["password"] = "password 至少包含字母和数字"
	}

	if len(fields) > 0 {
		return authsvc.RegisterCommand{}, validation.BodyValidationError(fields)
	}
	return command, nil
}

func parseLoginCommand(request *openapigen.LoginRequest) (authsvc.LoginCommand, *apperror.AppError) {
	if request == nil {
		return authsvc.LoginCommand{}, validation.MalformedBodyError()
	}

	fields := validation.FieldErrors{}
	command := authsvc.LoginCommand{
		UsernameOrEmail: strings.TrimSpace(request.UsernameOrEmail),
		Password:        request.Password,
	}

	if command.UsernameOrEmail == "" {
		fields["usernameOrEmail"] = "用户名或邮箱不能为空"
	} else if len(command.UsernameOrEmail) > 128 {
		fields["usernameOrEmail"] = "用户名或邮箱长度不能超过 128 个字符"
	}

	if command.Password == "" {
		fields["password"] = "密码不能为空"
	} else if len(command.Password) > 72 {
		fields["password"] = "密码长度不能超过 72 个字符"
	}

	if len(fields) > 0 {
		return authsvc.LoginCommand{}, validation.BodyValidationError(fields)
	}
	return command, nil
}

func parseRefreshCommand(request *openapigen.RefreshTokenRequest) (authsvc.RefreshCommand, *apperror.AppError) {
	if request == nil {
		return authsvc.RefreshCommand{}, validation.MalformedBodyError()
	}

	fields := validation.FieldErrors{}

	if strings.TrimSpace(request.RefreshToken) == "" {
		fields["refreshToken"] = "refreshToken 不能为空"
	} else if len(request.RefreshToken) > 128 {
		fields["refreshToken"] = "refreshToken 长度不能超过 128 个字符"
	}

	if len(fields) > 0 {
		return authsvc.RefreshCommand{}, validation.BodyValidationError(fields)
	}
	return authsvc.RefreshCommand{RefreshToken: request.RefreshToken}, nil
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
