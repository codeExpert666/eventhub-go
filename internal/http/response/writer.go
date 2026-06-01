package response

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"eventhub-go/internal/apperror"
)

// ContentTypeJSON 表示统一 JSON 响应使用的 Content-Type。
const ContentTypeJSON = "application/json; charset=utf-8"

// WriteSuccess 写出统一成功响应，HTTP 状态码固定为 200。
func WriteSuccess(w http.ResponseWriter, r *http.Request, data any) {
	WriteJSON(w, http.StatusOK, Success(r, data))
}

// WriteError 写出统一失败响应，HTTP 状态码由业务错误码映射得到。
// 当 err 为空时，默认按系统内部错误处理，避免向调用方写出空错误响应。
func WriteError(w http.ResponseWriter, r *http.Request, err *apperror.AppError) {
	if err == nil {
		err = apperror.New(apperror.CommonInternal, "")
	}
	WriteJSON(w, err.Code().HTTPStatus(), Failure(r, err))
}

// WriteStatus 只写出 HTTP 状态码和 JSON Content-Type，不写响应体。
func WriteStatus(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", ContentTypeJSON)
	w.WriteHeader(status)
}

// WriteJSON 按指定 HTTP 状态码写出 JSON 响应体。
// WriteHeader 会提交响应头；后续写入 body 后不能再变更状态码。
// 编码失败时仅记录日志；此时响应头和状态码可能已经写出，不能再切换为错误响应。
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", ContentTypeJSON)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Default().Error("failed to encode http response", "error", err)
	}
}
