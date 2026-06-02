// Package auth 提供认证模块 HTTP handler。
package auth

import (
	authsvc "eventhub-go/internal/service/auth"
)

// Handler 处理认证模块 HTTP 请求。
type Handler struct {
	auth *authsvc.Service
}

// NewHandler 创建认证 handler。
func NewHandler(auth *authsvc.Service) *Handler {
	return &Handler{auth: auth}
}
