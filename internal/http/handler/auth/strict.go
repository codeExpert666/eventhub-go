package auth

import (
	"context"
	"errors"

	openapigen "eventhub-go/api/openapi/gen"
	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/response"
	"eventhub-go/internal/security"
	authsvc "eventhub-go/internal/service/auth"
	usersvc "eventhub-go/internal/service/user"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

// RegisterStrict 将 generated strict register request 映射到 auth service。
func (h *Handler) RegisterStrict(ctx context.Context, request openapigen.RegisterRequestObject) (openapigen.RegisterResponseObject, error) {
	command, appErr := parseRegisterCommand(request.Body)
	if appErr != nil {
		return nil, appErr
	}
	result, err := h.auth.Register(ctx, command)
	if err != nil {
		return nil, apperror.FromErrorOrInternal(err)
	}
	base := response.SuccessMeta(ctx)
	return openapigen.Register200JSONResponse(openapigen.ApiResponseUserInfo{
		Code:      base.Code,
		Data:      toOpenAPIUserInfo(result),
		Message:   base.Message,
		RequestId: base.RequestID,
		Timestamp: base.Timestamp,
	}), nil
}

// LoginStrict 将 generated strict login request 映射到 auth service。
func (h *Handler) LoginStrict(ctx context.Context, request openapigen.LoginRequestObject) (openapigen.LoginResponseObject, error) {
	command, appErr := parseLoginCommand(request.Body)
	if appErr != nil {
		return nil, appErr
	}
	result, err := h.auth.Login(ctx, command)
	if err != nil {
		return nil, apperror.FromErrorOrInternal(err)
	}
	base := response.SuccessMeta(ctx)
	return openapigen.Login200JSONResponse(openapigen.ApiResponseLogin{
		Code:      base.Code,
		Data:      toOpenAPILoginResponse(result),
		Message:   base.Message,
		RequestId: base.RequestID,
		Timestamp: base.Timestamp,
	}), nil
}

// RefreshTokenStrict 将 generated strict refresh request 映射到 auth service。
func (h *Handler) RefreshTokenStrict(ctx context.Context, request openapigen.RefreshTokenRequestObject) (openapigen.RefreshTokenResponseObject, error) {
	command, appErr := parseRefreshCommand(request.Body)
	if appErr != nil {
		return nil, appErr
	}
	result, err := h.auth.Refresh(ctx, command)
	if err != nil {
		return nil, apperror.FromErrorOrInternal(err)
	}
	base := response.SuccessMeta(ctx)
	return openapigen.RefreshToken200JSONResponse(openapigen.ApiResponseTokenPair{
		Code:      base.Code,
		Data:      toOpenAPIRefreshTokenResponse(result),
		Message:   base.Message,
		RequestId: base.RequestID,
		Timestamp: base.Timestamp,
	}), nil
}

// LogoutStrict 根据认证主体执行登出。
func (h *Handler) LogoutStrict(ctx context.Context, _ openapigen.LogoutRequestObject) (openapigen.LogoutResponseObject, error) {
	principal, err := security.RequiredPrincipal(ctx)
	if err != nil {
		if errors.Is(err, security.ErrMissingPrincipal) {
			return nil, apperror.New(apperror.AuthUnauthorized, "请先登录或重新登录")
		}
		return nil, apperror.FromErrorOrInternal(err)
	}
	if err = h.auth.Logout(ctx, authsvc.LogoutCommand{Principal: principal}); err != nil {
		return nil, apperror.FromErrorOrInternal(err)
	}
	base := response.SuccessMeta(ctx)
	return openapigen.Logout200JSONResponse(openapigen.ApiResponseVoid{
		Code:      base.Code,
		Message:   base.Message,
		Data:      nil,
		RequestId: base.RequestID,
		Timestamp: base.Timestamp,
	}), nil
}

type tokenPairData struct {
	AccessToken         string
	RefreshToken        string
	AuthorizationScheme string
	ExpiresIn           int64
	RefreshExpiresIn    int64
	SessionID           string
	User                usersvc.UserResult
}

func toOpenAPILoginResponse(result authsvc.LoginResult) openapigen.LoginResponse {
	return openapigen.LoginResponse(toOpenAPITokenPairResponse(tokenPairData{
		AccessToken:         result.AccessToken,
		RefreshToken:        result.RefreshToken,
		AuthorizationScheme: result.AuthorizationScheme,
		ExpiresIn:           result.ExpiresIn,
		RefreshExpiresIn:    result.RefreshExpiresIn,
		SessionID:           result.SessionID,
		User:                result.User,
	}))
}

func toOpenAPIRefreshTokenResponse(result authsvc.RefreshResult) openapigen.TokenPairResponse {
	return toOpenAPITokenPairResponse(tokenPairData{
		AccessToken:         result.AccessToken,
		RefreshToken:        result.RefreshToken,
		AuthorizationScheme: result.AuthorizationScheme,
		ExpiresIn:           result.ExpiresIn,
		RefreshExpiresIn:    result.RefreshExpiresIn,
		SessionID:           result.SessionID,
		User:                result.User,
	})
}

func toOpenAPITokenPairResponse(data tokenPairData) openapigen.TokenPairResponse {
	return openapigen.TokenPairResponse{
		AccessToken:         data.AccessToken,
		RefreshToken:        data.RefreshToken,
		AuthorizationScheme: data.AuthorizationScheme,
		ExpiresIn:           data.ExpiresIn,
		RefreshExpiresIn:    data.RefreshExpiresIn,
		SessionId:           data.SessionID,
		User:                toOpenAPIUserInfo(data.User),
	}
}

func toOpenAPIUserInfo(result usersvc.UserResult) openapigen.UserInfo {
	roles := result.Roles
	if roles == nil {
		roles = []string{}
	}
	return openapigen.UserInfo{
		Id:       result.ID,
		Username: result.Username,
		Email:    openapi_types.Email(result.Email),
		Status:   openapigen.UserStatus(result.Status),
		Roles:    roles,
	}
}
