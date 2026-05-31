package response

import (
	"net/http"
	"time"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/platform/idgen"
)

// APIResponse is the unified response envelope for business APIs.
type APIResponse struct {
	Code      string    `json:"code"`
	Message   string    `json:"message"`
	Data      any       `json:"data"`
	RequestID string    `json:"requestId"`
	Timestamp time.Time `json:"timestamp"`
}

// Success builds a successful APIResponse.
func Success(r *http.Request, data any) APIResponse {
	return APIResponse{
		Code:      apperror.CommonSuccess.String(),
		Message:   apperror.CommonSuccess.DefaultMessage(),
		Data:      data,
		RequestID: idgen.RequestIDFromContext(r.Context()),
		Timestamp: time.Now(),
	}
}

// Failure builds a failed APIResponse from an application error.
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
