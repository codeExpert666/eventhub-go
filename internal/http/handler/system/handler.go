// Package system 将 system service 的用例适配为 HTTP handler。
package system

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"eventhub-go/internal/apperror"
	systemdto "eventhub-go/internal/http/dto/system"
	"eventhub-go/internal/http/response"
	"eventhub-go/internal/http/validation"
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

// Ping 使用统一响应 envelope 写出基础服务存活信息。
func (h *Handler) Ping(w http.ResponseWriter, r *http.Request) {
	result := h.service.Ping(r.Context())
	response.WriteSuccess(w, r, systemdto.PingResponse{
		ServiceName:    result.ServiceName,
		ActiveProfiles: result.ActiveProfiles,
		ServerTime:     result.ServerTime,
	})
}

// Echo 校验请求体，并返回回显消息与服务端时间。
func (h *Handler) Echo(w http.ResponseWriter, r *http.Request) {
	var request systemdto.EchoRequest
	if err := validation.DecodeJSONBody(r, &request); err != nil {
		response.WriteError(w, r, err)
		return
	}

	if err := validateEchoRequest(request); err != nil {
		response.WriteError(w, r, err)
		return
	}

	result := h.service.Echo(r.Context(), systemsvc.EchoCommand{
		Message: request.Message,
		Tag:     request.Tag,
	})
	response.WriteSuccess(w, r, systemdto.EchoResponse{
		Message:  result.Message,
		Tag:      result.Tag,
		EchoedAt: result.EchoedAt,
	})
}

// Health 写出兼容 actuator 的最小健康检查响应。
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	result := h.service.Health(r.Context())
	response.WriteJSON(w, http.StatusOK, systemdto.HealthResponse{Status: result.Status})
}

// HealthHead 为 actuator 健康检查探针写出仅包含状态码的响应。
func (h *Handler) HealthHead(w http.ResponseWriter, r *http.Request) {
	response.WriteStatus(w, http.StatusOK)
}

// Info 写出兼容 actuator 的应用信息与运行时元数据。
func (h *Handler) Info(w http.ResponseWriter, r *http.Request) {
	result := h.service.Info(r.Context())
	response.WriteJSON(w, http.StatusOK, systemdto.InfoResponse{
		App: systemdto.AppInfoResponse{
			Name:           result.App.Name,
			Env:            result.App.Env,
			Version:        result.App.Version,
			ActiveProfiles: result.App.ActiveProfiles,
		},
		Runtime: systemdto.RuntimeInfoResponse{ServerTime: result.Runtime.ServerTime},
	})
}

// InfoHead 为 actuator 信息探针写出仅包含状态码的响应。
func (h *Handler) InfoHead(w http.ResponseWriter, r *http.Request) {
	response.WriteStatus(w, http.StatusOK)
}

// validateEchoRequest 校验 system echo 端点的 HTTP 请求契约。
func validateEchoRequest(request systemdto.EchoRequest) *apperror.AppError {
	fields := validation.FieldErrors{}
	if strings.TrimSpace(request.Message) == "" {
		fields["message"] = "message 不能为空"
	} else if utf8.RuneCountInString(request.Message) > 64 {
		fields["message"] = "message 长度不能超过 64"
	}

	if request.Tag != nil && utf8.RuneCountInString(*request.Tag) > 32 {
		fields["tag"] = "tag 长度不能超过 32"
	}

	if len(fields) > 0 {
		return validation.BodyValidationError(fields)
	}
	return nil
}
