package auth

import "context"

// Logout 表达已认证用户登出语义。当前 access token 无状态，服务端不修改 DB。
func (s *Service) Logout(ctx context.Context, command LogoutCommand) error {
	if command.Principal.UserID <= 0 {
		return missingPrincipalError()
	}
	return nil
}
