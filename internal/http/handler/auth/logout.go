package auth

import (
	"net/http"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/response"
	"eventhub-go/internal/http/validation"
	"eventhub-go/internal/security"
	authsvc "eventhub-go/internal/service/auth"
)

// Logout 处理 POST /api/v1/auth/logout。
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	principal, err := security.RequiredPrincipal(r.Context())
	if err != nil {
		response.WriteError(w, r, apperror.New(apperror.AuthUnauthorized, "请先登录或重新登录"))
		return
	}
	if err := h.auth.Logout(r.Context(), authsvc.LogoutCommand{Principal: principal}); err != nil {
		response.WriteError(w, r, validation.AppErrorFromError(err))
		return
	}
	response.WriteSuccess(w, r, nil)
}
