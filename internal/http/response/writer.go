package response

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"eventhub-go/internal/apperror"
)

// ContentTypeJSON is the content type used for JSON responses.
const ContentTypeJSON = "application/json; charset=utf-8"

// WriteSuccess writes a successful unified API response.
func WriteSuccess(w http.ResponseWriter, r *http.Request, data any) {
	WriteJSON(w, http.StatusOK, Success(r, data))
}

// WriteError writes a failed unified API response.
func WriteError(w http.ResponseWriter, r *http.Request, err *apperror.AppError) {
	if err == nil {
		err = apperror.New(apperror.CommonInternal, "")
	}
	WriteJSON(w, err.Code().HTTPStatus(), Failure(r, err))
}

// WriteStatus writes only a status code and JSON content type.
func WriteStatus(w http.ResponseWriter, status int) {
	w.Header().Set("Content-Type", ContentTypeJSON)
	w.WriteHeader(status)
}

// WriteJSON writes a JSON response with the provided HTTP status.
func WriteJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", ContentTypeJSON)
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Default().Error("failed to encode http response", "error", err)
	}
}
