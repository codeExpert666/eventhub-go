// Package user 提供当前用户 HTTP handler。
package user

import (
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
