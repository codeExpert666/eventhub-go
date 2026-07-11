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
	violations := requesterror.Violations{}
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
	query.CreatedAtFrom = parseTimeParam(params.CreatedAtFrom, "createdAtFrom", &violations)
	query.CreatedAtTo = parseTimeParam(params.CreatedAtTo, "createdAtTo", &violations)
	query.UpdatedAtFrom = parseTimeParam(params.UpdatedAtFrom, "updatedAtFrom", &violations)
	query.UpdatedAtTo = parseTimeParam(params.UpdatedAtTo, "updatedAtTo", &violations)

	if len(query.Username) > 32 {
		violations = append(violations, queryViolation("username", "maxLength", "用户名筛选长度不能超过 32"))
	}
	if len(query.Email) > 128 {
		violations = append(violations, queryViolation("email", "maxLength", "邮箱筛选长度不能超过 128"))
	}
	if query.CreatedAtFrom != nil && query.CreatedAtTo != nil && query.CreatedAtFrom.After(*query.CreatedAtTo) {
		violations = append(violations, queryViolation("createdAtFrom", "notAfter", "createdAtFrom 不能晚于 createdAtTo"))
	}
	if query.UpdatedAtFrom != nil && query.UpdatedAtTo != nil && query.UpdatedAtFrom.After(*query.UpdatedAtTo) {
		violations = append(violations, queryViolation("updatedAtFrom", "notAfter", "updatedAtFrom 不能晚于 updatedAtTo"))
	}

	if len(violations) > 0 {
		return usersvc.AdminUserListQuery{}, requesterror.InvalidParameters(violations)
	}
	return query, nil
}

func parseUpdateUserStatusCommand(userID int64, request *openapigen.UpdateUserStatusRequest) (usersvc.UpdateUserStatusCommand, *apperror.AppError) {
	if userID <= 0 {
		return usersvc.UpdateUserStatusCommand{}, requesterror.InvalidParameters(requesterror.Violations{{
			Location: requesterror.LocationPath,
			Field:    "userId",
			Path:     "userId",
			Rule:     "minimum",
			Message:  "userId 必须是正整数",
		}})
	}
	if request == nil {
		return usersvc.UpdateUserStatusCommand{}, requesterror.MissingBody()
	}

	return usersvc.UpdateUserStatusCommand{
		UserID: userID,
		Status: string(request.Status),
	}, nil
}

func parseTimeParam(rawValue *string, name string, violations *requesterror.Violations) *time.Time {
	if rawValue == nil {
		return nil
	}
	raw := strings.TrimSpace(*rawValue)
	if raw == "" {
		return nil
	}
	parsed, err := time.Parse(localDateTimeLayout, raw)
	if err != nil {
		*violations = append(*violations, queryViolation(name, "pattern", name+" 格式不合法"))
		return nil
	}
	return &parsed
}

func queryViolation(field, rule, message string) requesterror.Violation {
	return requesterror.Violation{
		Location: requesterror.LocationQuery,
		Field:    field,
		Path:     field,
		Rule:     rule,
		Message:  message,
	}
}
