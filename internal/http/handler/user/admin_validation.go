package user

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"eventhub-go/internal/apperror"
	userdto "eventhub-go/internal/http/dto/user"
	"eventhub-go/internal/http/validation"
	"eventhub-go/internal/page"
)

const localDateTimeLayout = "2006-01-02T15:04:05"

func parseAdminUserListRequest(values url.Values) (userdto.AdminUserListRequest, *apperror.AppError) {
	fields := validation.FieldErrors{}
	request := userdto.AdminUserListRequest{
		Page: page.DefaultPage,
		Size: page.DefaultSize,
	}

	request.Page = parseIntQuery(values, "page", page.DefaultPage, fields)
	request.Size = parseIntQuery(values, "size", page.DefaultSize, fields)
	request.Username = values.Get("username")
	request.Email = values.Get("email")
	request.Status = values.Get("status")
	request.CreatedAtFrom = parseTimeQuery(values, "createdAtFrom", fields)
	request.CreatedAtTo = parseTimeQuery(values, "createdAtTo", fields)
	request.UpdatedAtFrom = parseTimeQuery(values, "updatedAtFrom", fields)
	request.UpdatedAtTo = parseTimeQuery(values, "updatedAtTo", fields)

	validateAdminUserListRequest(request, fields)
	if len(fields) > 0 {
		return userdto.AdminUserListRequest{}, queryValidationError(fields)
	}
	return request, nil
}

func validateAdminUserListRequest(request userdto.AdminUserListRequest, fields validation.FieldErrors) {
	if request.Page < 1 {
		fields["page"] = "页码不能小于 1"
	}
	if request.Size < 1 {
		fields["size"] = "每页条数不能小于 1"
	} else if request.Size > page.MaxSize {
		fields["size"] = "每页条数不能超过 100"
	}
	if len(strings.TrimSpace(request.Username)) > 32 {
		fields["username"] = "用户名筛选长度不能超过 32"
	}
	if len(strings.TrimSpace(request.Email)) > 128 {
		fields["email"] = "邮箱筛选长度不能超过 128"
	}
	status := strings.TrimSpace(request.Status)
	if status != "" && status != string(userdto.UserStatusEnabled) && status != string(userdto.UserStatusDisabled) {
		fields["status"] = "用户状态只能是 ENABLED 或 DISABLED"
	}
	if request.CreatedAtFrom != nil && request.CreatedAtTo != nil && request.CreatedAtFrom.After(*request.CreatedAtTo) {
		fields["createdAtFrom"] = "createdAtFrom 不能晚于 createdAtTo"
	}
	if request.UpdatedAtFrom != nil && request.UpdatedAtTo != nil && request.UpdatedAtFrom.After(*request.UpdatedAtTo) {
		fields["updatedAtFrom"] = "updatedAtFrom 不能晚于 updatedAtTo"
	}
}

func validateUpdateUserStatusRequest(request userdto.UpdateUserStatusRequest) *apperror.AppError {
	fields := validation.FieldErrors{}
	if request.Status == nil {
		fields["status"] = "status 不能为空"
	}
	if len(fields) > 0 {
		return validation.BodyValidationError(fields)
	}
	return nil
}

func parseIntQuery(values url.Values, name string, defaultValue int, fields validation.FieldErrors) int {
	raw := strings.TrimSpace(values.Get(name))
	if raw == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		fields[name] = name + " 必须是整数"
		return defaultValue
	}
	return value
}

func parseTimeQuery(values url.Values, name string, fields validation.FieldErrors) *time.Time {
	raw := strings.TrimSpace(values.Get(name))
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

func queryValidationError(fields validation.FieldErrors) *apperror.AppError {
	return apperror.WithData(
		apperror.CommonValidation,
		"请求参数校验失败",
		fields,
	)
}
