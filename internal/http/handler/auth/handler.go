// Package auth 提供认证模块 HTTP handler。
package auth

import (
	"context"

	authsvc "eventhub-go/internal/service/auth"
	usersvc "eventhub-go/internal/service/user"
)

// AuthService 表示认证 handler 依赖的 service 契约。
type AuthService interface {
	Register(ctx context.Context, command authsvc.RegisterCommand) (usersvc.UserResult, error)
	Login(ctx context.Context, command authsvc.LoginCommand) (authsvc.LoginResult, error)
}

// Handler 处理认证模块 HTTP 请求。
type Handler struct {
	auth AuthService
}

// NewHandler 创建认证 handler。
func NewHandler(auth AuthService) *Handler {
	return &Handler{auth: auth}
}
