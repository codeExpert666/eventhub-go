package user

import (
	"context"
	"strings"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/repository"
	"eventhub-go/internal/security"
)

const roleAuthorityPrefix = "ROLE_"

// LoadPrincipal 按用户 ID 加载最新用户状态和角色，并构造认证主体。
func (s *Service) LoadPrincipal(ctx context.Context, userID int64) (security.Principal, error) {
	user, found, err := s.users.FindByID(ctx, userID)
	if err != nil {
		return security.Principal{}, err
	}
	if !found || user.Status != repository.UserStatusEnabled {
		return security.Principal{}, apperror.New(apperror.AuthUnauthorized, "请先登录或重新登录")
	}
	roleCodes, err := s.roles.FindRoleCodesByUserID(ctx, user.ID)
	if err != nil {
		return security.Principal{}, err
	}
	return security.Principal{
		UserID:      user.ID,
		Username:    user.Username,
		Authorities: toAuthorities(roleCodes),
	}, nil
}

func toAuthorities(roleCodes []string) []string {
	if len(roleCodes) == 0 {
		return []string{}
	}
	authorities := make([]string, 0, len(roleCodes))
	for _, roleCode := range roleCodes {
		if strings.HasPrefix(roleCode, roleAuthorityPrefix) {
			authorities = append(authorities, roleCode)
			continue
		}
		authorities = append(authorities, roleAuthorityPrefix+roleCode)
	}
	return authorities
}
