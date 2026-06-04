// Package auth 定义认证模块 HTTP 请求 DTO。
package auth

// RegisterRequest 表示用户注册请求体。
type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginRequest 表示用户登录请求体。
type LoginRequest struct {
	UsernameOrEmail string `json:"usernameOrEmail"`
	Password        string `json:"password"`
}

// RefreshTokenRequest 表示 refresh token 续期请求体。
type RefreshTokenRequest struct {
	RefreshToken string `json:"refreshToken"`
}
