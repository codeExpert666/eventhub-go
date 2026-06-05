package user

import (
	"context"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/repository"
)

// CurrentUser 根据认证主体查询当前用户资料。
func (s *Service) CurrentUser(ctx context.Context, query CurrentUserQuery) (UserResult, error) {
	if query.Principal.UserID <= 0 {
		return UserResult{}, apperror.New(apperror.AuthUnauthorized, "请先登录或重新登录")
	}
	return s.GetByID(ctx, query.Principal.UserID)
}

// GetByID 按用户 ID 查询用户摘要。
func (s *Service) GetByID(ctx context.Context, userID int64) (UserResult, error) {
	user, found, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return UserResult{}, err
	}
	if !found {
		return UserResult{}, apperror.New(apperror.CommonNotFound, "用户不存在")
	}
	roles, err := s.roles.FindRoleCodesByUserID(ctx, user.ID)
	if err != nil {
		return UserResult{}, err
	}
	if roles == nil {
		roles = []string{}
	}
	return toUserResult(user, roles), nil
}

func toUserResult(user repository.User, roles []string) UserResult {
	if roles == nil {
		roles = []string{}
	}
	return UserResult{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Status:   string(user.Status),
		Roles:    append([]string(nil), roles...),
	}
}
