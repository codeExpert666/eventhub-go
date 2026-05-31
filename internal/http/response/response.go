package response

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/requestid"
)

const ContentTypeJSON = "application/json; charset=utf-8"

type APIResponse struct {
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Data      any       `json:"data"`
	RequestID string    `json:"requestId"`
	Timestamp time.Time `json:"timestamp"`
}

func Success(r *http.Request, data any) APIResponse {
	return APIResponse{
		Code:      apperror.CommonSuccess.String(),
		Message:   apperror.CommonSuccess.DefaultMessage(),
		Data:      data,
		RequestID: requestid.FromContext(r.Context()),
		Timestamp: time.Now(),
	}
}

func Failure(r *http.Request, err *apperror.AppError) APIResponse {
	if err == nil {
		err = apperror.New(apperror.CommonInternal, "")
	}
	return APIResponse{
		Code:      err.Code().String(),
		Message:   err.Message(),
		Data:      err.Data(),
		RequestID: requestid.FromContext(r.Context()),
		Timestamp: time.Now(),
	}
}

func WriteSuccess(w http.ResponseWriter, r *http.Request, data any) {
	WriteJSON(w, http.StatusOK, Success(r, data))
}

func WriteError(w http.ResponseWriter, r *http.Request, err *apperror.AppError) {
	if err == nil {
		err = apperror.New(apperror.CommonInternal, "")
	}
	WriteJSON(w, err.Code().HTTPStatus(), Failure(r, err))
}

func WriteStatus(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", ContentTypeJSON)
	w.WriteHeader(status)
}

func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", ContentTypeJSON)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Default().Error("failed to encode http response", "error", err)
	}
}
