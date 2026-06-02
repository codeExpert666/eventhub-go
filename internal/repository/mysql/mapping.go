package mysql

import (
	"database/sql"
	"time"

	"eventhub-go/internal/repository"
	"eventhub-go/internal/repository/mysql/sqlc"
)

func mapUser(row sqlc.User) repository.User {
	return repository.User{
		ID:           row.ID,
		Username:     row.Username,
		Email:        row.Email,
		PasswordHash: row.PasswordHash,
		Status:       repository.UserStatus(row.Status),
		CreatedAt:    row.CreatedAt,
		UpdatedAt:    row.UpdatedAt,
	}
}

func mapRole(row sqlc.Role) repository.Role {
	return repository.Role{
		ID:          row.ID,
		Code:        row.Code,
		Name:        row.Name,
		Description: nullStringPtr(row.Description),
		CreatedAt:   row.CreatedAt,
	}
}

func mapAuthSession(row sqlc.AuthSession) repository.AuthSession {
	return repository.AuthSession{
		ID:               row.ID,
		SessionID:        row.SessionID,
		UserID:           row.UserID,
		RefreshTokenHash: row.RefreshTokenHash,
		Status:           repository.AuthSessionStatus(row.Status),
		IssuedAt:         row.IssuedAt,
		RefreshExpiresAt: row.RefreshExpiresAt,
		LastRefreshedAt:  nullTimePtr(row.LastRefreshedAt),
		LastSeenAt:       nullTimePtr(row.LastSeenAt),
		RevokedAt:        nullTimePtr(row.RevokedAt),
		RevokeReason:     nullStringPtr(row.RevokeReason),
		ClientIPHash:     nullStringPtr(row.ClientIpHash),
		UserAgentHash:    nullStringPtr(row.UserAgentHash),
		UserAgentSummary: nullStringPtr(row.UserAgentSummary),
		Version:          row.Version,
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}

func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

func nullableString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

func nullableTime(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *value, Valid: true}
}

func requiredNullableTime(value time.Time) sql.NullTime {
	return sql.NullTime{Time: value, Valid: true}
}
