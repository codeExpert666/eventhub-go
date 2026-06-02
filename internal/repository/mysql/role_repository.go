package mysql

import (
	"context"
	"database/sql"
	"errors"

	"eventhub-go/internal/repository"
	"eventhub-go/internal/repository/mysql/sqlc"
)

type RoleRepository struct {
	factory queryFactory
}

func NewRoleRepository(database *sql.DB) *RoleRepository {
	return &RoleRepository{factory: queryFactory{db: database}}
}

func (r *RoleRepository) FindByCode(ctx context.Context, code string) (repository.Role, bool, error) {
	row, err := r.factory.queries(ctx).FindRoleByCode(ctx, code)
	if errors.Is(err, sql.ErrNoRows) {
		return repository.Role{}, false, nil
	}
	if err != nil {
		return repository.Role{}, false, err
	}
	return mapRole(row), true, nil
}

func (r *RoleRepository) FindRoleCodesByUserID(ctx context.Context, userID int64) ([]string, error) {
	codes, err := r.factory.queries(ctx).FindRoleCodesByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if codes == nil {
		return []string{}, nil
	}
	return codes, nil
}

func (r *RoleRepository) FindRoleCodesByUserIDs(ctx context.Context, userIDs []int64) ([]repository.UserRoleCode, error) {
	if len(userIDs) == 0 {
		return []repository.UserRoleCode{}, nil
	}
	rows, err := r.factory.queries(ctx).FindRoleCodesByUserIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	codes := make([]repository.UserRoleCode, 0, len(rows))
	for _, row := range rows {
		codes = append(codes, repository.UserRoleCode{
			UserID:   row.UserID,
			RoleCode: row.RoleCode,
		})
	}
	return codes, nil
}

func (r *RoleRepository) AddRoleToUser(ctx context.Context, userID, roleID int64) (int64, error) {
	return rowsAffected(r.factory.queries(ctx).AddRoleToUser(ctx, sqlc.AddRoleToUserParams{
		UserID: userID,
		RoleID: roleID,
	}))
}
