package user

import (
	"errors"
	"net/http"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/response"
	"eventhub-go/internal/http/validation"
	"eventhub-go/internal/security"
	usersvc "eventhub-go/internal/service/user"
)

// Me 处理 GET /api/v1/me。
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	principal, err := principalFromContext(r.Context())
	if err != nil {
		if errors.Is(err, security.ErrMissingPrincipal) {
			response.WriteError(w, r, apperror.New(apperror.AuthUnauthorized, "请先登录或重新登录"))
			return
		}
		response.WriteError(w, r, validation.AppErrorFromError(err))
		return
	}
	result, err := h.users.CurrentUser(r.Context(), usersvc.CurrentUserQuery{Principal: principal})
	if err != nil {
		response.WriteError(w, r, validation.AppErrorFromError(err))
		return
	}
	response.WriteSuccess(w, r, toUserInfoResponse(result))
}
