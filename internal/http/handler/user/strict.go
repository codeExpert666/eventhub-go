package user

import (
	"context"
	"errors"

	openapigen "eventhub-go/api/openapi/gen"
	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/response"
	"eventhub-go/internal/page"
	"eventhub-go/internal/security"
	usersvc "eventhub-go/internal/service/user"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

// GetCurrentUserStrict 根据认证主体返回当前用户。
func (h *Handler) GetCurrentUserStrict(ctx context.Context, _ openapigen.GetCurrentUserRequestObject) (openapigen.GetCurrentUserResponseObject, error) {
	principal, err := security.RequiredPrincipal(ctx)
	if err != nil {
		if errors.Is(err, security.ErrMissingPrincipal) {
			return nil, apperror.New(apperror.AuthUnauthorized, "请先登录或重新登录")
		}
		return nil, apperror.FromErrorOrInternal(err)
	}
	result, err := h.users.CurrentUser(ctx, usersvc.CurrentUserQuery{Principal: principal})
	if err != nil {
		return nil, apperror.FromErrorOrInternal(err)
	}
	base := response.SuccessMeta(ctx)
	return openapigen.GetCurrentUser200JSONResponse(openapigen.ApiResponseUserInfo{
		Code:      base.Code,
		Data:      toOpenAPIUserInfo(result),
		Message:   base.Message,
		RequestId: base.RequestID,
		Timestamp: base.Timestamp,
	}), nil
}

// ListAdminUsersStrict 将 generated query params 映射到 user service 查询。
func (h *Handler) ListAdminUsersStrict(ctx context.Context, request openapigen.ListAdminUsersRequestObject) (openapigen.ListAdminUsersResponseObject, error) {
	query, appErr := parseAdminUserListQuery(request.Params)
	if appErr != nil {
		return nil, appErr
	}
	result, err := h.users.ListUsers(ctx, query)
	if err != nil {
		return nil, apperror.FromErrorOrInternal(err)
	}
	base := response.SuccessMeta(ctx)
	return openapigen.ListAdminUsers200JSONResponse(openapigen.ApiResponseAdminUserPage{
		Code:      base.Code,
		Data:      toOpenAPIAdminUserPage(result),
		Message:   base.Message,
		RequestId: base.RequestID,
		Timestamp: base.Timestamp,
	}), nil
}

// UpdateAdminUserStatusStrict 将 generated path/body 参数映射到 user service 命令。
func (h *Handler) UpdateAdminUserStatusStrict(ctx context.Context, request openapigen.UpdateAdminUserStatusRequestObject) (openapigen.UpdateAdminUserStatusResponseObject, error) {
	command, appErr := parseUpdateUserStatusCommand(request.UserId, request.Body)
	if appErr != nil {
		return nil, appErr
	}
	result, err := h.users.UpdateStatus(ctx, command)
	if err != nil {
		return nil, apperror.FromErrorOrInternal(err)
	}
	base := response.SuccessMeta(ctx)
	return openapigen.UpdateAdminUserStatus200JSONResponse(openapigen.ApiResponseUserInfo{
		Code:      base.Code,
		Data:      toOpenAPIUserInfo(result),
		Message:   base.Message,
		RequestId: base.RequestID,
		Timestamp: base.Timestamp,
	}), nil
}

func toOpenAPIAdminUserPage(result page.Response[usersvc.UserResult]) openapigen.PageResponseUserInfo {
	data := openapigen.PageResponseUserInfo{
		Items:       make([]openapigen.UserInfo, 0, len(result.Items)),
		Page:        result.Page,
		Size:        result.Size,
		Total:       result.Total,
		TotalPages:  result.TotalPages,
		HasNext:     result.HasNext,
		HasPrevious: result.HasPrevious,
	}
	for _, item := range result.Items {
		data.Items = append(data.Items, toOpenAPIUserInfo(item))
	}
	return data
}

func toOpenAPIUserInfo(result usersvc.UserResult) openapigen.UserInfo {
	roles := result.Roles
	if roles == nil {
		roles = []string{}
	}
	return openapigen.UserInfo{
		Id:       result.ID,
		Username: result.Username,
		Email:    openapi_types.Email(result.Email),
		Status:   openapigen.UserStatus(result.Status),
		Roles:    roles,
	}
}
