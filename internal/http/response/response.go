package response

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	openapigen "eventhub-go/api/openapi/gen"
	"eventhub-go/internal/apperror"
	"eventhub-go/internal/platform/idgen"
)

const contentTypeJSON = "application/json; charset=utf-8"

// Meta 表示业务 API 响应 envelope 中由项目统一生成的元数据。
type Meta struct {
	Code      string
	Message   string
	RequestID string
	Timestamp time.Time
}

// SuccessMeta 构造成功响应元数据，供 strict handler 填充 generated typed response。
func SuccessMeta(ctx context.Context) Meta {
	return Meta{
		Code:      apperror.CommonSuccess.String(),
		Message:   apperror.CommonSuccess.DefaultMessage(),
		RequestID: idgen.RequestIDFromContext(ctx),
		Timestamp: time.Now(),
	}
}

// WriteError 写出统一失败响应，HTTP 状态码由业务错误码映射得到。
// 当 err 为空时，默认按系统内部错误处理，避免向调用方写出空错误响应。
func WriteError(w http.ResponseWriter, r *http.Request, err *apperror.AppError) {
	appErr := apperror.FromErrorOrInternal(err)
	writeJSON(w, appErr.Code().HTTPStatus(), errorResponse(r.Context(), appErr))
}

// writeJSON 按指定 HTTP 状态码写出 JSON 响应体。
// WriteHeader 会提交响应头；后续写入 body 后不能再变更状态码。
// 编码失败时仅记录日志；此时响应头和状态码可能已经写出，不能再切换为错误响应。
func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", contentTypeJSON)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Default().Error("failed to encode http response", "error", err)
	}
}

func errorResponse(ctx context.Context, err *apperror.AppError) openapigen.ErrorResponse {
	return openapigen.ErrorResponse{
		Code:      err.Code().String(),
		Message:   err.Message(),
		Data:      detailsData(err.Details()),
		RequestId: idgen.RequestIDFromContext(ctx),
		Timestamp: time.Now(),
	}
}

func detailsData(details apperror.Details) *map[string]interface{} {
	if details == nil {
		return nil
	}
	data := map[string]interface{}(details)
	return &data
}
