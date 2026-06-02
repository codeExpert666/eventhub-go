// Package auth 承载注册、登录和 token 签发用例。
package auth

import (
	"context"
	"time"

	"eventhub-go/internal/platform/clock"
	"eventhub-go/internal/repository"
	usersvc "eventhub-go/internal/service/user"
)

// Transactor 表示 service 层需要的事务边界能力。
type Transactor interface {
	WithinTx(ctx context.Context, fn func(context.Context) error) error
}

// PasswordHasher 表示密码哈希和校验能力。
type PasswordHasher interface {
	Hash(plain string) (string, error)
	Matches(plain, hashed string) (bool, error)
}

// TokenIssuer 表示 access token 签发能力。
type TokenIssuer interface {
	IssueAccessToken(subjectID int64, sessionID string) (string, error)
	AccessTokenTTL() time.Duration
}

// RefreshTokenManager 表示 opaque refresh token 生成和哈希能力。
type RefreshTokenManager interface {
	Generate() (string, error)
	Hash(token string) (string, error)
	RefreshTokenTTL() time.Duration
}

// UserReader 表示按用户 ID 查询用户摘要的能力。
type UserReader interface {
	GetByID(ctx context.Context, userID int64) (usersvc.UserResult, error)
}

// Dependencies 聚合 auth service 依赖。
type Dependencies struct {
	Users        repository.UserRepository
	Roles        repository.RoleRepository
	Sessions     repository.AuthSessionRepository
	Transactor   Transactor
	Passwords    PasswordHasher
	Tokens       TokenIssuer
	RefreshToken RefreshTokenManager
	UserReader   UserReader
	Clock        clock.Clock
}

// Service 承载认证用例。
type Service struct {
	users        repository.UserRepository
	roles        repository.RoleRepository
	sessions     repository.AuthSessionRepository
	transactor   Transactor
	passwords    PasswordHasher
	tokens       TokenIssuer
	refreshToken RefreshTokenManager
	userReader   UserReader
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
		userReader:   deps.UserReader,
		clock:        serviceClock,
	}
}
