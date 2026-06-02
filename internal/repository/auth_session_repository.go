package repository

import (
	"context"
	"time"
)

type AuthSessionStatus string

const (
	AuthSessionStatusActive  AuthSessionStatus = "ACTIVE"
	AuthSessionStatusRevoked AuthSessionStatus = "REVOKED"
)

type AuthSession struct {
	ID               int64
	SessionID        string
	UserID           int64
	RefreshTokenHash string
	Status           AuthSessionStatus
	IssuedAt         time.Time
	RefreshExpiresAt time.Time
	LastRefreshedAt  *time.Time
	LastSeenAt       *time.Time
	RevokedAt        *time.Time
	RevokeReason     *string
	ClientIPHash     *string
	UserAgentHash    *string
	UserAgentSummary *string
	Version          int32
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type CreateAuthSessionInput struct {
	SessionID        string
	UserID           int64
	RefreshTokenHash string
	Status           AuthSessionStatus
	IssuedAt         time.Time
	RefreshExpiresAt time.Time
	LastRefreshedAt  *time.Time
	LastSeenAt       *time.Time
	RevokedAt        *time.Time
	RevokeReason     *string
	ClientIPHash     *string
	UserAgentHash    *string
	UserAgentSummary *string
	Version          int32
}

type RotateRefreshTokenInput struct {
	SessionID           string
	OldRefreshTokenHash string
	OldVersion          int32
	NewRefreshTokenHash string
	RefreshedAt         time.Time
	RefreshExpiresAt    time.Time
}

type RevokeAuthSessionInput struct {
	SessionID    string
	RevokedAt    time.Time
	RevokeReason string
}

type AuthSessionRepository interface {
	Create(ctx context.Context, input CreateAuthSessionInput) (AuthSession, error)
	FindBySessionID(ctx context.Context, sessionID string) (AuthSession, bool, error)
	FindByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (AuthSession, bool, error)
	RotateRefreshToken(ctx context.Context, input RotateRefreshTokenInput) (int64, error)
	UpdateLastSeenAt(ctx context.Context, sessionID string, lastSeenAt time.Time) (int64, error)
	RevokeBySessionID(ctx context.Context, input RevokeAuthSessionInput) (int64, error)
	UpdateStatus(ctx context.Context, sessionID string, status AuthSessionStatus) (int64, error)
}
