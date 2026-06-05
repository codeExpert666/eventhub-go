package user

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"eventhub-go/internal/apperror"
	userdto "eventhub-go/internal/http/dto/user"
	"eventhub-go/internal/http/response"
	"eventhub-go/internal/http/validation"
	usersvc "eventhub-go/internal/service/user"
)

// ListUsers 处理 GET /api/v1/admin/users。
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	request, appErr := parseAdminUserListRequest(r.URL.Query())
	if appErr != nil {
		response.WriteError(w, r, appErr)
		return
	}
	result, err := h.users.ListUsers(r.Context(), usersvc.AdminUserListQuery{
		Page:          request.Page,
		Size:          request.Size,
		Username:      request.Username,
		Email:         request.Email,
		Status:        request.Status,
		CreatedAtFrom: request.CreatedAtFrom,
		CreatedAtTo:   request.CreatedAtTo,
		UpdatedAtFrom: request.UpdatedAtFrom,
		UpdatedAtTo:   request.UpdatedAtTo,
	})
	if err != nil {
		response.WriteError(w, r, validation.AppErrorFromError(err))
		return
	}
	response.WriteSuccess(w, r, toUserInfoPageResponse(result))
}

// UpdateStatus 处理 PATCH /api/v1/admin/users/{userId}/status。
func (h *Handler) UpdateStatus(w http.ResponseWriter, r *http.Request) {
	userID, appErr := parseUserIDParam(chi.URLParam(r, "userId"))
	if appErr != nil {
		response.WriteError(w, r, appErr)
		return
	}

	var request userdto.UpdateUserStatusRequest
	if err := validation.DecodeJSONBody(r, &request); err != nil {
		response.WriteError(w, r, err)
		return
	}
	if err := validateUpdateUserStatusRequest(request); err != nil {
		response.WriteError(w, r, err)
		return
	}

	result, err := h.users.UpdateStatus(r.Context(), usersvc.UpdateUserStatusCommand{
		UserID: userID,
		Status: string(*request.Status),
	})
	if err != nil {
		response.WriteError(w, r, validation.AppErrorFromError(err))
		return
	}
	response.WriteSuccess(w, r, toUserInfoResponse(result))
}

func parseUserIDParam(raw string) (int64, *apperror.AppError) {
	userID, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || userID <= 0 {
		return 0, queryValidationError(validation.FieldErrors{
			"userId": "userId 必须是正整数",
		})
	}
	return userID, nil
}
