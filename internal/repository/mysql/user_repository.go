package mysql

import (
	"context"
	"database/sql"
	"errors"

	"eventhub-go/internal/repository"
	"eventhub-go/internal/repository/mysql/sqlc"
)

type UserRepository struct {
	factory queryFactory
}

func NewUserRepository(database *sql.DB) *UserRepository {
	return &UserRepository{factory: queryFactory{db: database}}
}

func (r *UserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	count, err := r.factory.queries(ctx).CountUsersByUsername(ctx, username)
	return count > 0, err
}

func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	count, err := r.factory.queries(ctx).CountUsersByEmail(ctx, email)
	return count > 0, err
}

func (r *UserRepository) Create(ctx context.Context, input repository.CreateUserInput) (repository.User, error) {
	id, err := lastInsertID(r.factory.queries(ctx).CreateUser(ctx, sqlc.CreateUserParams{
		Username:     input.Username,
		Email:        input.Email,
		PasswordHash: input.PasswordHash,
		Status:       string(input.Status),
	}))
	if err != nil {
		return repository.User{}, err
	}
	user, found, err := r.FindByID(ctx, id)
	if err != nil {
		return repository.User{}, err
	}
	if !found {
		return repository.User{}, sql.ErrNoRows
	}
	return user, nil
}

func (r *UserRepository) FindByUsernameOrEmail(
	ctx context.Context,
	usernameOrEmail string,
) (repository.User, bool, error) {
	row, err := r.factory.queries(ctx).FindUserByUsernameOrEmail(ctx, sqlc.FindUserByUsernameOrEmailParams{
		Username: usernameOrEmail,
		Email:    usernameOrEmail,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return repository.User{}, false, nil
	}
	if err != nil {
		return repository.User{}, false, err
	}
	return mapUser(row), true, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id int64) (repository.User, bool, error) {
	row, err := r.factory.queries(ctx).FindUserByID(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return repository.User{}, false, nil
	}
	if err != nil {
		return repository.User{}, false, err
	}
	return mapUser(row), true, nil
}

func (r *UserRepository) CountByCriteria(ctx context.Context, criteria repository.UserCriteria) (int64, error) {
	return r.factory.queries(ctx).CountUsersByCriteria(ctx, sqlc.CountUsersByCriteriaParams{
		Username:      nullableFilter(criteria.Username),
		Email:         nullableFilter(criteria.Email),
		Status:        nullableStatus(criteria.Status),
		CreatedAtFrom: nullableTime(criteria.CreatedAtFrom),
		CreatedAtTo:   nullableTime(criteria.CreatedAtTo),
		UpdatedAtFrom: nullableTime(criteria.UpdatedAtFrom),
		UpdatedAtTo:   nullableTime(criteria.UpdatedAtTo),
	})
}

func (r *UserRepository) FindPage(
	ctx context.Context,
	criteria repository.UserCriteria,
	limit int32,
	offset int32,
) ([]repository.User, error) {
	rows, err := r.factory.queries(ctx).FindUsersPageByCriteria(ctx, sqlc.FindUsersPageByCriteriaParams{
		Username:      nullableFilter(criteria.Username),
		Email:         nullableFilter(criteria.Email),
		Status:        nullableStatus(criteria.Status),
		CreatedAtFrom: nullableTime(criteria.CreatedAtFrom),
		CreatedAtTo:   nullableTime(criteria.CreatedAtTo),
		UpdatedAtFrom: nullableTime(criteria.UpdatedAtFrom),
		UpdatedAtTo:   nullableTime(criteria.UpdatedAtTo),
		Limit:         limit,
		Offset:        offset,
	})
	if err != nil {
		return nil, err
	}
	users := make([]repository.User, 0, len(rows))
	for _, row := range rows {
		users = append(users, mapUser(row))
	}
	return users, nil
}

func (r *UserRepository) UpdateStatus(ctx context.Context, id int64, status repository.UserStatus) (int64, error) {
	return rowsAffected(r.factory.queries(ctx).UpdateUserStatus(ctx, sqlc.UpdateUserStatusParams{
		ID:     id,
		Status: string(status),
	}))
}

func nullableFilter(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func nullableStatus(status *repository.UserStatus) sql.NullString {
	if status == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: string(*status), Valid: true}
}
