// Package system 将 system service 的用例适配为 HTTP handler。
package system

import (
	systemsvc "eventhub-go/internal/service/system"
)

// Handler 将 system service 的用例适配为 HTTP 处理器。
type Handler struct {
	service *systemsvc.Service
}

// NewHandler 创建 system 与 actuator 端点使用的处理器。
func NewHandler(service *systemsvc.Service) *Handler {
	return &Handler{service: service}
}
