package mysql

import (
	"context"
	"database/sql"
	"errors"

	"eventhub-go/internal/repository"
	"eventhub-go/internal/repository/mysql/sqlc"
)

// UserRepository 是基于 MySQL 和 sqlc 的用户仓储实现。
type UserRepository struct {
	// factory 负责根据 context 中是否存在事务选择正确的 sqlc 执行器。
	factory queryFactory
}

// NewUserRepository 使用指定数据库连接池创建 MySQL 用户仓储。
func NewUserRepository(database *sql.DB) *UserRepository {
	return &UserRepository{factory: queryFactory{db: database}}
}

// ExistsByUsername 判断指定用户名是否已存在。
func (r *UserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	count, err := r.factory.queries(ctx).CountUsersByUsername(ctx, username)
	return count > 0, err
}

// ExistsByEmail 判断指定邮箱是否已存在。
func (r *UserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	count, err := r.factory.queries(ctx).CountUsersByEmail(ctx, email)
	return count > 0, err
}

// Create 创建用户并返回数据库中的完整用户快照。
//
// 插入成功后会按自增 ID 重新查询一次，确保返回值包含数据库默认值和触发器等持久化结果。
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

// FindByUsernameOrEmail 按用户名或邮箱查找用户。
//
// 第二个返回值表示是否命中；未命中时返回空用户、false 和 nil error。
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

// FindByID 按用户 ID 查找用户。
//
// 第二个返回值表示是否命中；未命中时返回空用户、false 和 nil error。
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

// CountByCriteria 按筛选条件统计用户数量。
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

// FindPage 按筛选条件分页查询用户列表，并将 sqlc 行模型映射为 repository 层模型。
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

// UpdateStatus 更新用户状态，并返回受影响行数。
func (r *UserRepository) UpdateStatus(ctx context.Context, id int64, status repository.UserStatus) (int64, error) {
	return rowsAffected(r.factory.queries(ctx).UpdateUserStatus(ctx, sqlc.UpdateUserStatusParams{
		ID:     id,
		Status: string(status),
	}))
}
