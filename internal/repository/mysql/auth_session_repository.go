package mysql

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"eventhub-go/internal/repository"
	"eventhub-go/internal/repository/mysql/sqlc"
)

type AuthSessionRepository struct {
	factory queryFactory
}

func NewAuthSessionRepository(database *sql.DB) *AuthSessionRepository {
	return &AuthSessionRepository{factory: queryFactory{db: database}}
}

func (r *AuthSessionRepository) Create(
	ctx context.Context,
	input repository.CreateAuthSessionInput,
) (repository.AuthSession, error) {
	id, err := lastInsertID(r.factory.queries(ctx).CreateAuthSession(ctx, sqlc.CreateAuthSessionParams{
		SessionID:        input.SessionID,
		UserID:           input.UserID,
		RefreshTokenHash: input.RefreshTokenHash,
		Status:           string(input.Status),
		IssuedAt:         input.IssuedAt,
		RefreshExpiresAt: input.RefreshExpiresAt,
		LastRefreshedAt:  nullableTime(input.LastRefreshedAt),
		LastSeenAt:       nullableTime(input.LastSeenAt),
		RevokedAt:        nullableTime(input.RevokedAt),
		RevokeReason:     nullableString(input.RevokeReason),
		ClientIpHash:     nullableString(input.ClientIPHash),
		UserAgentHash:    nullableString(input.UserAgentHash),
		UserAgentSummary: nullableString(input.UserAgentSummary),
		Version:          input.Version,
	}))
	if err != nil {
		return repository.AuthSession{}, err
	}
	session, found, err := r.findByIDFallback(ctx, id, input.SessionID)
	if err != nil {
		return repository.AuthSession{}, err
	}
	if !found {
		return repository.AuthSession{}, sql.ErrNoRows
	}
	return session, nil
}

func (r *AuthSessionRepository) FindBySessionID(
	ctx context.Context,
	sessionID string,
) (repository.AuthSession, bool, error) {
	row, err := r.factory.queries(ctx).FindAuthSessionBySessionID(ctx, sessionID)
	if errors.Is(err, sql.ErrNoRows) {
		return repository.AuthSession{}, false, nil
	}
	if err != nil {
		return repository.AuthSession{}, false, err
	}
	return mapAuthSession(row), true, nil
}

func (r *AuthSessionRepository) FindByRefreshTokenHash(
	ctx context.Context,
	refreshTokenHash string,
) (repository.AuthSession, bool, error) {
	row, err := r.factory.queries(ctx).FindAuthSessionByRefreshTokenHash(ctx, refreshTokenHash)
	if errors.Is(err, sql.ErrNoRows) {
		return repository.AuthSession{}, false, nil
	}
	if err != nil {
		return repository.AuthSession{}, false, err
	}
	return mapAuthSession(row), true, nil
}

func (r *AuthSessionRepository) ConditionalRotate(
	ctx context.Context,
	input repository.ConditionalRotateAuthSessionInput,
) (int64, error) {
	return rowsAffected(r.factory.queries(ctx).ConditionalRotateAuthSessionRefreshToken(ctx, sqlc.ConditionalRotateAuthSessionRefreshTokenParams{
		SessionID:          input.SessionID,
		RefreshTokenHash:   input.NewRefreshTokenHash,
		RefreshTokenHash_2: input.OldRefreshTokenHash,
		Version:            input.OldVersion,
		LastRefreshedAt:    requiredNullableTime(input.RefreshedAt),
		LastSeenAt:         requiredNullableTime(input.RefreshedAt),
		RefreshExpiresAt:   input.RefreshExpiresAt,
		RefreshExpiresAt_2: input.RefreshedAt,
	}))
}

func (r *AuthSessionRepository) UpdateLastSeenAt(
	ctx context.Context,
	sessionID string,
	lastSeenAt time.Time,
) (int64, error) {
	return rowsAffected(r.factory.queries(ctx).UpdateAuthSessionLastSeenAt(ctx, sqlc.UpdateAuthSessionLastSeenAtParams{
		SessionID:  sessionID,
		LastSeenAt: requiredNullableTime(lastSeenAt),
	}))
}

func (r *AuthSessionRepository) RevokeBySessionID(
	ctx context.Context,
	input repository.RevokeAuthSessionInput,
) (int64, error) {
	return rowsAffected(r.factory.queries(ctx).RevokeAuthSessionBySessionID(ctx, sqlc.RevokeAuthSessionBySessionIDParams{
		SessionID:    input.SessionID,
		RevokedAt:    requiredNullableTime(input.RevokedAt),
		RevokeReason: sql.NullString{String: input.RevokeReason, Valid: input.RevokeReason != ""},
	}))
}

func (r *AuthSessionRepository) UpdateStatus(
	ctx context.Context,
	sessionID string,
	status repository.AuthSessionStatus,
) (int64, error) {
	return rowsAffected(r.factory.queries(ctx).UpdateAuthSessionStatus(ctx, sqlc.UpdateAuthSessionStatusParams{
		SessionID: sessionID,
		Status:    string(status),
	}))
}

func (r *AuthSessionRepository) findByIDFallback(
	ctx context.Context,
	_ int64,
	sessionID string,
) (repository.AuthSession, bool, error) {
	return r.FindBySessionID(ctx, sessionID)
}
