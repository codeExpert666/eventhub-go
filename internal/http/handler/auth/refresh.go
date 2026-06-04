package auth

import (
	"net/http"

	authdto "eventhub-go/internal/http/dto/auth"
	"eventhub-go/internal/http/response"
	"eventhub-go/internal/http/validation"
	authsvc "eventhub-go/internal/service/auth"
)

// Refresh 处理 POST /api/v1/auth/refresh。
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var request authdto.RefreshTokenRequest
	if err := validation.DecodeJSONBody(r, &request); err != nil {
		response.WriteError(w, r, err)
		return
	}
	if err := validateRefreshTokenRequest(request); err != nil {
		response.WriteError(w, r, err)
		return
	}
	result, err := h.auth.Refresh(r.Context(), authsvc.RefreshCommand{
		RefreshToken: request.RefreshToken,
	})
	if err != nil {
		response.WriteError(w, r, validation.AppErrorFromError(err))
		return
	}
	response.WriteSuccess(w, r, toTokenPairResponse(result))
}
