package system

import (
	"context"

	openapigen "eventhub-go/api/openapi/gen"
	"eventhub-go/internal/http/response"
)

// PingStrict 将 generated strict request 映射到 system service。
func (h *Handler) PingStrict(ctx context.Context, _ openapigen.PingRequestObject) (openapigen.PingResponseObject, error) {
	result := h.service.Ping(ctx)
	base := response.SuccessMeta(ctx)
	return openapigen.Ping200JSONResponse(openapigen.ApiResponsePing{
		Code: base.Code,
		Data: openapigen.PingResponse{
			ActiveProfiles: result.ActiveProfiles,
			ServerTime:     result.ServerTime,
			ServiceName:    result.ServiceName,
		},
		Message:   base.Message,
		RequestId: base.RequestID,
		Timestamp: base.Timestamp,
	}), nil
}

// EchoStrict 校验 generated strict request body，并返回回显响应。
func (h *Handler) EchoStrict(ctx context.Context, request openapigen.EchoRequestObject) (openapigen.EchoResponseObject, error) {
	command, appErr := parseEchoCommand(request.Body)
	if appErr != nil {
		return nil, appErr
	}

	result := h.service.Echo(ctx, command)
	base := response.SuccessMeta(ctx)
	return openapigen.Echo200JSONResponse(openapigen.ApiResponseEcho{
		Code: base.Code,
		Data: openapigen.EchoResponse{
			Message:  result.Message,
			Tag:      result.Tag,
			EchoedAt: result.EchoedAt,
		},
		Message:   base.Message,
		RequestId: base.RequestID,
		Timestamp: base.Timestamp,
	}), nil
}

// HealthStrict 写出 actuator 健康检查响应。
func (h *Handler) HealthStrict(ctx context.Context, _ openapigen.HealthRequestObject) (openapigen.HealthResponseObject, error) {
	result := h.service.Health(ctx)
	return openapigen.Health200JSONResponse{Status: result.Status}, nil
}

// HealthHeadStrict 写出 actuator 健康检查 HEAD 响应。
func (h *Handler) HealthHeadStrict(context.Context, openapigen.HealthHeadRequestObject) (openapigen.HealthHeadResponseObject, error) {
	return openapigen.HealthHead200Response{}, nil
}

// InfoStrict 写出 actuator 应用信息响应。
func (h *Handler) InfoStrict(ctx context.Context, _ openapigen.InfoRequestObject) (openapigen.InfoResponseObject, error) {
	result := h.service.Info(ctx)
	return openapigen.Info200JSONResponse{
		App: openapigen.AppInfoResponse{
			Name:           result.App.Name,
			Env:            openapigen.AppInfoResponseEnv(result.App.Env),
			Version:        result.App.Version,
			ActiveProfiles: result.App.ActiveProfiles,
		},
		Runtime: openapigen.RuntimeInfoResponse{ServerTime: result.Runtime.ServerTime},
	}, nil
}

// InfoHeadStrict 写出 actuator info HEAD 响应。
func (h *Handler) InfoHeadStrict(context.Context, openapigen.InfoHeadRequestObject) (openapigen.InfoHeadResponseObject, error) {
	return openapigen.InfoHead200Response{}, nil
}
