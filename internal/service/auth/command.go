package auth

import "eventhub-go/internal/security"

// RegisterCommand 表示注册用户的写入输入。
type RegisterCommand struct {
	Username string
	Email    string
	Password string
}

// LoginCommand 表示登录用户的写入输入。
type LoginCommand struct {
	UsernameOrEmail string
	Password        string
}

// RefreshCommand 表示 refresh token 轮换输入。
type RefreshCommand struct {
	RefreshToken string
}

// LogoutCommand 表示登出输入。
type LogoutCommand struct {
	Principal security.Principal
}
