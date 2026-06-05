package user

import (
	"time"

	"eventhub-go/internal/security"
)

// CurrentUserQuery 表示查询当前用户资料所需的认证主体。
type CurrentUserQuery struct {
	Principal security.Principal
}

// AdminUserListQuery 表示管理员分页查询用户列表的读操作输入。
type AdminUserListQuery struct {
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
