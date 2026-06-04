// Package auth 承载注册、登录和 token 签发用例。
package auth

import (
	"errors"
	"time"

	"eventhub-go/internal/platform/clock"
	platformdb "eventhub-go/internal/platform/db"
	"eventhub-go/internal/repository"
	"eventhub-go/internal/security/jwt"
	"eventhub-go/internal/security/password"
	"eventhub-go/internal/security/refresh"
	usersvc "eventhub-go/internal/service/user"
)

// Service 承载认证用例。
type Service struct {
	users        repository.UserRepository
	roles        repository.RoleRepository
	sessions     repository.AuthSessionRepository
	transactor   platformdb.TxRunner
	passwords    *password.BCryptHasher
	tokens       *jwt.Codec
	accessTTL    time.Duration
	refreshToken *refresh.Manager
	userService  *usersvc.Service
	clock        clock.Clock
}

// NewService 创建 auth service。
func NewService(
	users repository.UserRepository,
	roles repository.RoleRepository,
	sessions repository.AuthSessionRepository,
	transactor platformdb.TxRunner,
	passwords *password.BCryptHasher,
	tokens *jwt.Codec,
	accessTokenTTL time.Duration,
	refreshToken *refresh.Manager,
	userService *usersvc.Service,
	serviceClock clock.Clock,
) (*Service, error) {
	if accessTokenTTL <= 0 {
		return nil, errors.New("auth access token ttl must be positive")
	}
	if serviceClock == nil {
		serviceClock = clock.RealClock{}
	}
	return &Service{
		users:        users,
		roles:        roles,
		sessions:     sessions,
		transactor:   transactor,
		passwords:    passwords,
		tokens:       tokens,
		accessTTL:    accessTokenTTL,
		refreshToken: refreshToken,
		userService:  userService,
		clock:        serviceClock,
	}, nil
}
