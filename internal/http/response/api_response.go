package response

import (
	"net/http"
	"time"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/platform/idgen"
)

// APIResponse 表示业务 API 的统一响应信封。
// 所有 HTTP 业务响应都通过该结构稳定表达业务码、消息、数据、请求 ID 和响应时间。
type APIResponse struct {
	// Code 表示稳定的业务结果码，成功和失败都使用 apperror 中定义的错误码体系。
	Code string `json:"code"`
	// Message 表示面向调用方的结果说明。
	Message string `json:"message"`
	// Data 表示业务响应数据，失败时可携带结构化错误上下文。
	Data any `json:"data"`
	// RequestID 表示当前请求的追踪标识，来源于请求上下文。
	RequestID string `json:"requestId"`
	// Timestamp 表示构造响应时的服务端时间。
	Timestamp time.Time `json:"timestamp"`
}

// Success 构造统一成功响应。
// 响应码和默认消息来自通用成功错误码，requestId 从请求上下文读取。
func Success(r *http.Request, data any) APIResponse {
	return APIResponse{
		Code:      apperror.CommonSuccess.String(),
		Message:   apperror.CommonSuccess.DefaultMessage(),
		Data:      data,
		RequestID: idgen.RequestIDFromContext(r.Context()),
		Timestamp: time.Now(),
	}
}

// Failure 根据应用错误构造统一失败响应。
// 当 err 为空时，默认按系统内部错误处理，保证失败响应始终包含稳定错误码和消息。
func Failure(r *http.Request, err *apperror.AppError) APIResponse {
	if err == nil {
		err = apperror.New(apperror.CommonInternal, "")
	}
	return APIResponse{
		Code:      err.Code().String(),
		Message:   err.Message(),
		Data:      err.Data(),
		RequestID: idgen.RequestIDFromContext(r.Context()),
		Timestamp: time.Now(),
	}
}
