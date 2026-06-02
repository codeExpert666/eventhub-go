package mysql

import (
	"database/sql"
	"time"

	"eventhub-go/internal/repository"
	"eventhub-go/internal/repository/mysql/sqlc"
)

// mapUser 将 sqlc 用户行模型映射为 repository 层用户模型。
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

// mapRole 将 sqlc 角色行模型映射为 repository 层角色模型。
func mapRole(row sqlc.Role) repository.Role {
	return repository.Role{
		ID:          row.ID,
		Code:        row.Code,
		Name:        row.Name,
		Description: nullStringPtr(row.Description),
		CreatedAt:   row.CreatedAt,
	}
}

// mapAuthSession 将 sqlc 认证会话行模型映射为 repository 层认证会话模型。
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

// nullStringPtr 将数据库可空字符串转换为业务层使用的字符串指针。
//
// 当 sql.NullString.Valid 为 false 时返回 nil，表示数据库值为 NULL。
func nullStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

// nullTimePtr 将数据库可空时间转换为业务层使用的时间指针。
//
// 当 sql.NullTime.Valid 为 false 时返回 nil，表示数据库值为 NULL。
func nullTimePtr(value sql.NullTime) *time.Time {
	if !value.Valid {
		return nil
	}
	return &value.Time
}

// nullableString 将业务层字符串指针转换为 sqlc 查询参数需要的可空字符串。
//
// 当入参为 nil 时返回无效的 sql.NullString，使数据库写入或查询参数表达 NULL。
func nullableString(value *string) sql.NullString {
	if value == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *value, Valid: true}
}

// nullableFilter 将空字符串转换为 nil，使动态 SQL 条件忽略对应筛选项。
func nullableFilter(value string) any {
	if value == "" {
		return nil
	}
	return value
}

// nullableStatus 将可选用户状态转换为 sql.NullString，供 sqlc 查询参数使用。
func nullableStatus(status *repository.UserStatus) sql.NullString {
	if status == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: string(*status), Valid: true}
}

// nullableTime 将业务层时间指针转换为 sqlc 查询参数需要的可空时间。
//
// 当入参为 nil 时返回无效的 sql.NullTime，使数据库写入或查询参数表达 NULL。
func nullableTime(value *time.Time) sql.NullTime {
	if value == nil {
		return sql.NullTime{}
	}
	return sql.NullTime{Time: *value, Valid: true}
}

// requiredNullableTime 将必填时间包装为有效的 sql.NullTime，供 sqlc 可空参数复用。
func requiredNullableTime(value time.Time) sql.NullTime {
	return sql.NullTime{Time: value, Valid: true}
}
