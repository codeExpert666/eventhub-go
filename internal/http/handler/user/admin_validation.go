package user

import (
	"fmt"
	"strings"
	"time"

	openapigen "eventhub-go/api/openapi/gen"
	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/requesterror"
	"eventhub-go/internal/page"
	usersvc "eventhub-go/internal/service/user"
)

const localDateTimeLayout = "2006-01-02T15:04:05"

func parseAdminUserListQuery(params openapigen.ListAdminUsersParams) (usersvc.AdminUserListQuery, *apperror.AppError) {
	query := usersvc.AdminUserListQuery{
		Page: page.DefaultPage,
		Size: page.DefaultSize,
	}

	if params.Page != nil {
		query.Page = *params.Page
	}
	if params.Size != nil {
		query.Size = *params.Size
	}
	if params.Username != nil {
		query.Username = strings.TrimSpace(*params.Username)
	}
	if params.Email != nil {
		query.Email = strings.TrimSpace(*params.Email)
	}
	if params.Status != nil {
		query.Status = strings.TrimSpace(string(*params.Status))
	}

	var err error
	query.CreatedAtFrom, err = parseTimeParam(params.CreatedAtFrom, "createdAtFrom")
	if err != nil {
		return usersvc.AdminUserListQuery{}, apperror.FromErrorOrInternal(err)
	}
	query.CreatedAtTo, err = parseTimeParam(params.CreatedAtTo, "createdAtTo")
	if err != nil {
		return usersvc.AdminUserListQuery{}, apperror.FromErrorOrInternal(err)
	}
	query.UpdatedAtFrom, err = parseTimeParam(params.UpdatedAtFrom, "updatedAtFrom")
	if err != nil {
		return usersvc.AdminUserListQuery{}, apperror.FromErrorOrInternal(err)
	}
	query.UpdatedAtTo, err = parseTimeParam(params.UpdatedAtTo, "updatedAtTo")
	if err != nil {
		return usersvc.AdminUserListQuery{}, apperror.FromErrorOrInternal(err)
	}
	return query, nil
}

func parseUpdateUserStatusCommand(userID int64, request *openapigen.UpdateUserStatusRequest) (usersvc.UpdateUserStatusCommand, *apperror.AppError) {
	if request == nil {
		return usersvc.UpdateUserStatusCommand{}, requesterror.MalformedBody()
	}

	return usersvc.UpdateUserStatusCommand{
		UserID: userID,
		Status: string(request.Status),
	}, nil
}

func parseTimeParam(rawValue *string, name string) (*time.Time, error) {
	if rawValue == nil {
		return nil, nil
	}
	raw := strings.TrimSpace(*rawValue)
	if raw == "" {
		return nil, nil
	}
	parsed, err := time.Parse(localDateTimeLayout, raw)
	if err != nil {
		return nil, fmt.Errorf("parse OpenAPI-validated query parameter %s: %w", name, err)
	}
	return &parsed, nil
}
