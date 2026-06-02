// Package repository 定义业务层可依赖的持久化语义接口。
package repository

import (
	"context"
	"time"
)

type UserStatus string

const (
	UserStatusEnabled  UserStatus = "ENABLED"
	UserStatusDisabled UserStatus = "DISABLED"
)

type User struct {
	ID           int64
	Username     string
	Email        string
	PasswordHash string
	Status       UserStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CreateUserInput struct {
	Username     string
	Email        string
	PasswordHash string
	Status       UserStatus
}

type UserCriteria struct {
	Username      string
	Email         string
	Status        *UserStatus
	CreatedAtFrom *time.Time
	CreatedAtTo   *time.Time
	UpdatedAtFrom *time.Time
	UpdatedAtTo   *time.Time
}

type UserRepository interface {
	ExistsByUsername(ctx context.Context, username string) (bool, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	Create(ctx context.Context, input CreateUserInput) (User, error)
	FindByUsernameOrEmail(ctx context.Context, usernameOrEmail string) (User, bool, error)
	FindByID(ctx context.Context, id int64) (User, bool, error)
	CountByCriteria(ctx context.Context, criteria UserCriteria) (int64, error)
	FindPage(ctx context.Context, criteria UserCriteria, limit int32, offset int32) ([]User, error)
	UpdateStatus(ctx context.Context, id int64, status UserStatus) (int64, error)
}
