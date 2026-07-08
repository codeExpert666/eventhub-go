package user

import (
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
	fields := requesterror.FieldErrors{}
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
	query.CreatedAtFrom = parseTimeParam(params.CreatedAtFrom, "createdAtFrom", fields)
	query.CreatedAtTo = parseTimeParam(params.CreatedAtTo, "createdAtTo", fields)
	query.UpdatedAtFrom = parseTimeParam(params.UpdatedAtFrom, "updatedAtFrom", fields)
	query.UpdatedAtTo = parseTimeParam(params.UpdatedAtTo, "updatedAtTo", fields)

	if len(query.Username) > 32 {
		fields["username"] = "用户名筛选长度不能超过 32"
	}
	if len(query.Email) > 128 {
		fields["email"] = "邮箱筛选长度不能超过 128"
	}
	if query.CreatedAtFrom != nil && query.CreatedAtTo != nil && query.CreatedAtFrom.After(*query.CreatedAtTo) {
		fields["createdAtFrom"] = "createdAtFrom 不能晚于 createdAtTo"
	}
	if query.UpdatedAtFrom != nil && query.UpdatedAtTo != nil && query.UpdatedAtFrom.After(*query.UpdatedAtTo) {
		fields["updatedAtFrom"] = "updatedAtFrom 不能晚于 updatedAtTo"
	}

	if len(fields) > 0 {
		return usersvc.AdminUserListQuery{}, requesterror.InvalidParameters(fields)
	}
	return query, nil
}

func parseUpdateUserStatusCommand(userID int64, request *openapigen.UpdateUserStatusRequest) (usersvc.UpdateUserStatusCommand, *apperror.AppError) {
	if userID <= 0 {
		return usersvc.UpdateUserStatusCommand{}, requesterror.InvalidParameters(requesterror.FieldErrors{
			"userId": "userId 必须是正整数",
		})
	}
	if request == nil {
		return usersvc.UpdateUserStatusCommand{}, requesterror.MalformedBody()
	}

	return usersvc.UpdateUserStatusCommand{
		UserID: userID,
		Status: string(request.Status),
	}, nil
}

func parseTimeParam(rawValue *string, name string, fields requesterror.FieldErrors) *time.Time {
	if rawValue == nil {
		return nil
	}
	raw := strings.TrimSpace(*rawValue)
	if raw == "" {
		return nil
	}
	parsed, err := time.Parse(localDateTimeLayout, raw)
	if err != nil {
		fields[name] = name + " 格式不合法"
		return nil
	}
	return &parsed
}
