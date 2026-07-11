package auth

import (
	"strings"

	openapigen "eventhub-go/api/openapi/gen"
	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/requesterror"
	authsvc "eventhub-go/internal/service/auth"
)

func parseRegisterCommand(request *openapigen.RegisterRequest) (authsvc.RegisterCommand, *apperror.AppError) {
	if request == nil {
		return authsvc.RegisterCommand{}, requesterror.MalformedBody()
	}

	return authsvc.RegisterCommand{
		Username: strings.TrimSpace(request.Username),
		Email:    strings.TrimSpace(string(request.Email)),
		Password: request.Password,
	}, nil
}

func parseLoginCommand(request *openapigen.LoginRequest) (authsvc.LoginCommand, *apperror.AppError) {
	if request == nil {
		return authsvc.LoginCommand{}, requesterror.MalformedBody()
	}

	return authsvc.LoginCommand{
		UsernameOrEmail: strings.TrimSpace(request.UsernameOrEmail),
		Password:        request.Password,
	}, nil
}

func parseRefreshCommand(request *openapigen.RefreshTokenRequest) (authsvc.RefreshCommand, *apperror.AppError) {
	if request == nil {
		return authsvc.RefreshCommand{}, requesterror.MalformedBody()
	}

	return authsvc.RefreshCommand{RefreshToken: request.RefreshToken}, nil
}
