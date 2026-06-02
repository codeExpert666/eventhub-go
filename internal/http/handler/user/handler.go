// Package user 提供当前用户 HTTP handler。
package user

import (
	"context"

	"eventhub-go/internal/security"
	usersvc "eventhub-go/internal/service/user"
)

// Handler 处理当前用户 HTTP 请求。
type Handler struct {
	users *usersvc.Service
}

// NewHandler 创建当前用户 handler。
func NewHandler(users *usersvc.Service) *Handler {
	return &Handler{users: users}
}

func principalFromContext(ctx context.Context) (security.Principal, error) {
	return security.RequiredPrincipal(ctx)
}
