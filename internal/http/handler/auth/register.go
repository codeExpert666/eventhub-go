package auth

import (
	"net/http"

	authdto "eventhub-go/internal/http/dto/auth"
	"eventhub-go/internal/http/response"
	"eventhub-go/internal/http/validation"
	authsvc "eventhub-go/internal/service/auth"
)

// Register 处理 POST /api/v1/auth/register。
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var request authdto.RegisterRequest
	if err := validation.DecodeJSONBody(r, &request); err != nil {
		response.WriteError(w, r, err)
		return
	}
	if err := validateRegisterRequest(request); err != nil {
		response.WriteError(w, r, err)
		return
	}
	result, err := h.auth.Register(r.Context(), authsvc.RegisterCommand{
		Username: request.Username,
		Email:    request.Email,
		Password: request.Password,
	})
	if err != nil {
		response.WriteError(w, r, validation.AppErrorFromError(err))
		return
	}
	response.WriteSuccess(w, r, toUserInfoResponse(result))
}
