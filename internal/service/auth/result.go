package auth

import usersvc "eventhub-go/internal/service/user"

// LoginResult 表示登录成功后返回给 handler 的业务结果。
type LoginResult struct {
	AccessToken         string
	RefreshToken        string
	AuthorizationScheme string
	ExpiresIn           int64
	RefreshExpiresIn    int64
	SessionID           string
	User                usersvc.UserResult
}

// RefreshResult 表示 refresh 成功后返回给 handler 的业务结果。
type RefreshResult struct {
	AccessToken         string
	RefreshToken        string
	AuthorizationScheme string
	ExpiresIn           int64
	RefreshExpiresIn    int64
	SessionID           string
	User                usersvc.UserResult
}
