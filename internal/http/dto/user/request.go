package user

import (
	"encoding/json"
	"fmt"
	"time"
)

// AdminUserListRequest 表示管理员用户列表查询参数。
type AdminUserListRequest struct {
	Page          int
	Size          int
	Username      string
	Email         string
	Status        string
	CreatedAtFrom *time.Time
	CreatedAtTo   *time.Time
	UpdatedAtFrom *time.Time
	UpdatedAtTo   *time.Time
}

// UserStatusValue 表示 HTTP 请求体中允许提交的用户状态值。
type UserStatusValue string

const (
	// UserStatusEnabled 表示启用状态。
	UserStatusEnabled UserStatusValue = "ENABLED"
	// UserStatusDisabled 表示禁用状态。
	UserStatusDisabled UserStatusValue = "DISABLED"
)

// UnmarshalJSON 严格限制状态 JSON 值，避免非法枚举或数字 ordinal 进入业务层。
func (s *UserStatusValue) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	switch value {
	case string(UserStatusEnabled), string(UserStatusDisabled):
		*s = UserStatusValue(value)
		return nil
	default:
		return fmt.Errorf("unsupported user status: %s", value)
	}
}

// UpdateUserStatusRequest 表示管理员更新用户状态请求体。
type UpdateUserStatusRequest struct {
	Status *UserStatusValue `json:"status"`
}
