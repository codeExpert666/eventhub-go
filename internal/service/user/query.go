package user

import "eventhub-go/internal/security"

// CurrentUserQuery 表示查询当前用户资料所需的认证主体。
type CurrentUserQuery struct {
	Principal security.Principal
}
