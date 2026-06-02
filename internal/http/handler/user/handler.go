// Package user 提供当前用户 HTTP handler。
package user

import (
	"context"

	"eventhub-go/internal/security"
	usersvc "eventhub-go/internal/service/user"
)

// UserService 表示当前用户 handler 依赖的 service 契约。
type UserService interface {
	CurrentUser(ctx context.Context, query usersvc.CurrentUserQuery) (usersvc.UserResult, error)
}

// Handler 处理当前用户 HTTP 请求。
type Handler struct {
	users UserService
}

// NewHandler 创建当前用户 handler。
func NewHandler(users UserService) *Handler {
	return &Handler{users: users}
}

func principalFromContext(ctx context.Context) (security.Principal, error) {
	return security.RequiredPrincipal(ctx)
}
