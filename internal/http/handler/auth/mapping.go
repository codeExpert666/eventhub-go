package auth

import (
	authdto "eventhub-go/internal/http/dto/auth"
	userdto "eventhub-go/internal/http/dto/user"
	authsvc "eventhub-go/internal/service/auth"
	usersvc "eventhub-go/internal/service/user"
)

func toUserInfoResponse(result usersvc.UserResult) userdto.UserInfoResponse {
	roles := result.Roles
	if roles == nil {
		roles = []string{}
	}
	return userdto.UserInfoResponse{
		ID:       result.ID,
		Username: result.Username,
		Email:    result.Email,
		Status:   result.Status,
		Roles:    roles,
	}
}

func toLoginResponse(result authsvc.LoginResult) authdto.LoginResponse {
	return authdto.LoginResponse{
		AccessToken:         result.AccessToken,
		RefreshToken:        result.RefreshToken,
		AuthorizationScheme: result.AuthorizationScheme,
		ExpiresIn:           result.ExpiresIn,
		RefreshExpiresIn:    result.RefreshExpiresIn,
		SessionID:           result.SessionID,
		User:                toUserInfoResponse(result.User),
	}
}
