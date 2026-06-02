package auth

import (
	"net/http"

	authdto "eventhub-go/internal/http/dto/auth"
	"eventhub-go/internal/http/response"
	"eventhub-go/internal/http/validation"
	authsvc "eventhub-go/internal/service/auth"
)

// Login 处理 POST /api/v1/auth/login。
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var request authdto.LoginRequest
	if err := validation.DecodeJSONBody(r, &request); err != nil {
		response.WriteError(w, r, err)
		return
	}
	if err := validateLoginRequest(request); err != nil {
		response.WriteError(w, r, err)
		return
	}
	result, err := h.auth.Login(r.Context(), authsvc.LoginCommand{
		UsernameOrEmail: request.UsernameOrEmail,
		Password:        request.Password,
	})
	if err != nil {
		response.WriteError(w, r, validation.AppErrorFromError(err))
		return
	}
	response.WriteSuccess(w, r, toLoginResponse(result))
}
