// Package user 承载当前用户资料查询和认证主体加载用例。
package user

import (
	platformdb "eventhub-go/internal/platform/db"
	"eventhub-go/internal/repository"
)

// Service 聚合当前用户相关用例依赖。
type Service struct {
	users      repository.UserRepository
	roles      repository.RoleRepository
	transactor platformdb.TxRunner
}

// NewService 创建当前用户服务。
func NewService(users repository.UserRepository, roles repository.RoleRepository, transactor platformdb.TxRunner) *Service {
	return &Service{users: users, roles: roles, transactor: transactor}
}
