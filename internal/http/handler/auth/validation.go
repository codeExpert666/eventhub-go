package auth

import (
	"net/mail"
	"regexp"
	"strings"

	openapigen "eventhub-go/api/openapi/gen"
	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/requesterror"
	authsvc "eventhub-go/internal/service/auth"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

func parseRegisterCommand(request *openapigen.RegisterRequest) (authsvc.RegisterCommand, *apperror.AppError) {
	if request == nil {
		return authsvc.RegisterCommand{}, requesterror.MissingBody()
	}

	violations := requesterror.Violations{}
	command := authsvc.RegisterCommand{
		Username: strings.TrimSpace(request.Username),
		Email:    strings.TrimSpace(string(request.Email)),
		Password: request.Password,
	}

	if command.Username == "" {
		violations = append(violations, bodyViolation("username", "notBlank", "username 不能为空"))
	} else if len(command.Username) < 3 {
		violations = append(violations, bodyViolation("username", "minLength", "username 长度必须在 3 到 32 个字符之间"))
	} else if len(command.Username) > 32 {
		violations = append(violations, bodyViolation("username", "maxLength", "username 长度必须在 3 到 32 个字符之间"))
	} else if !usernamePattern.MatchString(command.Username) {
		violations = append(violations, bodyViolation("username", "pattern", "username 只能包含字母、数字和下划线"))
	}

	if command.Email == "" {
		violations = append(violations, bodyViolation("email", "notBlank", "email 不能为空"))
	} else if len(command.Email) > 128 {
		violations = append(violations, bodyViolation("email", "maxLength", "email 长度不能超过 128 个字符"))
	} else if _, err := mail.ParseAddress(command.Email); err != nil {
		violations = append(violations, bodyViolation("email", "format", "email 格式不合法"))
	}

	if command.Password == "" {
		violations = append(violations, bodyViolation("password", "notBlank", "password 不能为空"))
	} else if len(command.Password) < 8 {
		violations = append(violations, bodyViolation("password", "minLength", "password 长度必须在 8 到 72 个字符之间"))
	} else if len(command.Password) > 72 {
		violations = append(violations, bodyViolation("password", "maxLength", "password 长度必须在 8 到 72 个字符之间"))
	} else if !containsLetterAndDigit(command.Password) {
		violations = append(violations, bodyViolation("password", "containsLetterAndDigit", "password 至少包含字母和数字"))
	}

	if len(violations) > 0 {
		return authsvc.RegisterCommand{}, requesterror.InvalidBody(violations)
	}
	return command, nil
}

func parseLoginCommand(request *openapigen.LoginRequest) (authsvc.LoginCommand, *apperror.AppError) {
	if request == nil {
		return authsvc.LoginCommand{}, requesterror.MissingBody()
	}

	violations := requesterror.Violations{}
	command := authsvc.LoginCommand{
		UsernameOrEmail: strings.TrimSpace(request.UsernameOrEmail),
		Password:        request.Password,
	}

	if command.UsernameOrEmail == "" {
		violations = append(violations, bodyViolation("usernameOrEmail", "notBlank", "用户名或邮箱不能为空"))
	} else if len(command.UsernameOrEmail) > 128 {
		violations = append(violations, bodyViolation("usernameOrEmail", "maxLength", "用户名或邮箱长度不能超过 128 个字符"))
	}

	if command.Password == "" {
		violations = append(violations, bodyViolation("password", "notBlank", "密码不能为空"))
	} else if len(command.Password) > 72 {
		violations = append(violations, bodyViolation("password", "maxLength", "密码长度不能超过 72 个字符"))
	}

	if len(violations) > 0 {
		return authsvc.LoginCommand{}, requesterror.InvalidBody(violations)
	}
	return command, nil
}

func parseRefreshCommand(request *openapigen.RefreshTokenRequest) (authsvc.RefreshCommand, *apperror.AppError) {
	if request == nil {
		return authsvc.RefreshCommand{}, requesterror.MissingBody()
	}

	violations := requesterror.Violations{}

	if strings.TrimSpace(request.RefreshToken) == "" {
		violations = append(violations, bodyViolation("refreshToken", "notBlank", "refreshToken 不能为空"))
	} else if len(request.RefreshToken) > 128 {
		violations = append(violations, bodyViolation("refreshToken", "maxLength", "refreshToken 长度不能超过 128 个字符"))
	}

	if len(violations) > 0 {
		return authsvc.RefreshCommand{}, requesterror.InvalidBody(violations)
	}
	return authsvc.RefreshCommand{RefreshToken: request.RefreshToken}, nil
}

func bodyViolation(field, rule, message string) requesterror.Violation {
	return requesterror.Violation{
		Location: requesterror.LocationBody,
		Field:    field,
		Path:     field,
		Rule:     rule,
		Message:  message,
	}
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
