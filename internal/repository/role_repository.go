package repository

import (
	"context"
	"time"
)

type Role struct {
	ID          int64
	Code        string
	Name        string
	Description *string
	CreatedAt   time.Time
}

type UserRoleCode struct {
	UserID   int64
	RoleCode string
}

type RoleRepository interface {
	FindByCode(ctx context.Context, code string) (Role, bool, error)
	FindRoleCodesByUserID(ctx context.Context, userID int64) ([]string, error)
	FindRoleCodesByUserIDs(ctx context.Context, userIDs []int64) ([]UserRoleCode, error)
	AddRoleToUser(ctx context.Context, userID, roleID int64) (int64, error)
}
