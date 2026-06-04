package auth

import userdto "eventhub-go/internal/http/dto/user"

// LoginResponse 表示登录成功响应 data。
type LoginResponse struct {
	AccessToken         string                   `json:"accessToken"`
	RefreshToken        string                   `json:"refreshToken"`
	AuthorizationScheme string                   `json:"authorizationScheme"`
	ExpiresIn           int64                    `json:"expiresIn"`
	RefreshExpiresIn    int64                    `json:"refreshExpiresIn"`
	SessionID           string                   `json:"sessionId"`
	User                userdto.UserInfoResponse `json:"user"`
}

// TokenPairResponse 表示 refresh 成功响应 data。
type TokenPairResponse struct {
	AccessToken         string                   `json:"accessToken"`
	RefreshToken        string                   `json:"refreshToken"`
	AuthorizationScheme string                   `json:"authorizationScheme"`
	ExpiresIn           int64                    `json:"expiresIn"`
	RefreshExpiresIn    int64                    `json:"refreshExpiresIn"`
	SessionID           string                   `json:"sessionId"`
	User                userdto.UserInfoResponse `json:"user"`
}
