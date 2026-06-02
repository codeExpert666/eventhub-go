// Package auth 承载注册、登录和 token 签发用例。
package auth

import (
	"eventhub-go/internal/platform/clock"
	platformdb "eventhub-go/internal/platform/db"
	"eventhub-go/internal/repository"
	"eventhub-go/internal/security/jwt"
	"eventhub-go/internal/security/password"
	"eventhub-go/internal/security/refresh"
	usersvc "eventhub-go/internal/service/user"
)

// Dependencies 聚合 auth service 依赖。
type Dependencies struct {
	Users        repository.UserRepository
	Roles        repository.RoleRepository
	Sessions     repository.AuthSessionRepository
	Transactor   platformdb.TxRunner
	Passwords    *password.BCryptHasher
	Tokens       *jwt.Codec
	RefreshToken *refresh.Manager
	UserService  *usersvc.Service
	Clock        clock.Clock
}

// Service 承载认证用例。
type Service struct {
	users        repository.UserRepository
	roles        repository.RoleRepository
	sessions     repository.AuthSessionRepository
	transactor   platformdb.TxRunner
	passwords    *password.BCryptHasher
	tokens       *jwt.Codec
	refreshToken *refresh.Manager
	userService  *usersvc.Service
	clock        clock.Clock
}

// NewService 创建 auth service。
func NewService(deps Dependencies) *Service {
	serviceClock := deps.Clock
	if serviceClock == nil {
		serviceClock = clock.RealClock{}
	}
	return &Service{
		users:        deps.Users,
		roles:        deps.Roles,
		sessions:     deps.Sessions,
		transactor:   deps.Transactor,
		passwords:    deps.Passwords,
		tokens:       deps.Tokens,
		refreshToken: deps.RefreshToken,
		userService:  deps.UserService,
		clock:        serviceClock,
	}
}
