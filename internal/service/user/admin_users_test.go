package user

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/repository"
)

func TestListUsersUsesBatchRoleLoading(t *testing.T) {
	now := time.Now().UTC()
	users := &userServiceUserRepo{
		count: 2,
		page: []repository.User{
			{ID: 1, Username: "admin", Email: "admin@example.com", Status: repository.UserStatusEnabled, CreatedAt: now, UpdatedAt: now},
			{ID: 2, Username: "alice", Email: "alice@example.com", Status: repository.UserStatusEnabled, CreatedAt: now, UpdatedAt: now},
		},
	}
	roles := &userServiceRoleRepo{
		batchRows: []repository.UserRoleCode{
			{UserID: 1, RoleCode: "ADMIN"},
			{UserID: 1, RoleCode: "USER"},
			{UserID: 2, RoleCode: "USER"},
		},
	}
	service := NewService(users, roles, userServiceNoopTx{})

	result, err := service.ListUsers(context.Background(), AdminUserListQuery{Page: 1, Size: 20})
	if err != nil {
		t.Fatalf("list users: %v", err)
	}

	if result.Total != 2 || len(result.Items) != 2 {
		t.Fatalf("unexpected page result: %#v", result)
	}
	if roles.batchCalls != 1 {
		t.Fatalf("expected one batch role load, got %d", roles.batchCalls)
	}
	if roles.singleCalls != 0 {
		t.Fatalf("expected no single role loads, got %d", roles.singleCalls)
	}
	if len(result.Items[0].Roles) != 2 || result.Items[0].Roles[0] != "ADMIN" || result.Items[0].Roles[1] != "USER" {
		t.Fatalf("unexpected admin roles: %#v", result.Items[0].Roles)
	}
}

func TestListUsersSkipsRoleBatchWhenNoUsers(t *testing.T) {
	service := NewService(
		&userServiceUserRepo{count: 0},
		&userServiceRoleRepo{},
		userServiceNoopTx{},
	)

	result, err := service.ListUsers(context.Background(), AdminUserListQuery{Page: 1, Size: 20})
	if err != nil {
		t.Fatalf("list users: %v", err)
	}

	if result.Total != 0 || len(result.Items) != 0 {
		t.Fatalf("unexpected empty page: %#v", result)
	}
}

func TestListUsersRejectsPageOffsetOverflowBeforeCounting(t *testing.T) {
	users := &userServiceUserRepo{}
	service := NewService(
		users,
		&userServiceRoleRepo{},
		userServiceNoopTx{},
	)

	_, err := service.ListUsers(context.Background(), AdminUserListQuery{
		Page: int(math.MaxInt32),
		Size: 100,
	})

	appErr, ok := apperror.FromError(err)
	if !ok {
		t.Fatalf("expected app error, got %v", err)
	}
	if appErr.Code() != apperror.CommonValidation || appErr.Message() != "请求参数校验失败" {
		t.Fatalf("unexpected error: code=%s message=%s", appErr.Code(), appErr.Message())
	}
	if users.countCalls != 0 {
		t.Fatalf("expected offset validation before count, got %d count calls", users.countCalls)
	}
}

func TestUpdateStatusReturnsUpdatedUser(t *testing.T) {
	now := time.Now().UTC()
	users := &userServiceUserRepo{
		updateRows: 1,
		foundUser: repository.User{
			ID:        2,
			Username:  "alice",
			Email:     "alice@example.com",
			Status:    repository.UserStatusEnabled,
			CreatedAt: now,
			UpdatedAt: now,
		},
		found: true,
	}
	roles := &userServiceRoleRepo{rolesByUserID: map[int64][]string{2: []string{"USER"}}}
	tx := &userServiceCountingTx{}
	service := NewService(users, roles, tx)

	result, err := service.UpdateStatus(context.Background(), UpdateUserStatusCommand{
		UserID: 2,
		Status: "DISABLED",
	})
	if err != nil {
		t.Fatalf("update status: %v", err)
	}

	if result.Status != "DISABLED" || result.Roles[0] != "USER" {
		t.Fatalf("unexpected updated user: %#v", result)
	}
	if users.updatedStatus != repository.UserStatusDisabled {
		t.Fatalf("expected disabled update, got %s", users.updatedStatus)
	}
	if tx.calls != 1 {
		t.Fatalf("expected transaction to be used, got %d", tx.calls)
	}
}

func TestUpdateStatusReturnsNotFound(t *testing.T) {
	service := NewService(
		&userServiceUserRepo{updateRows: 0},
		&userServiceRoleRepo{},
		userServiceNoopTx{},
	)

	_, err := service.UpdateStatus(context.Background(), UpdateUserStatusCommand{
		UserID: 404,
		Status: "DISABLED",
	})

	appErr, ok := apperror.FromError(err)
	if !ok {
		t.Fatalf("expected app error, got %v", err)
	}
	if appErr.Code() != apperror.CommonNotFound || appErr.Message() != "用户不存在" {
		t.Fatalf("unexpected error: code=%s message=%s", appErr.Code(), appErr.Message())
	}
}

type userServiceNoopTx struct{}

func (userServiceNoopTx) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type userServiceCountingTx struct {
	calls int
}

func (tx *userServiceCountingTx) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	tx.calls++
	return fn(ctx)
}

type userServiceUserRepo struct {
	count         int64
	countCalls    int
	page          []repository.User
	updateRows    int64
	updatedStatus repository.UserStatus
	foundUser     repository.User
	found         bool
}

func (r *userServiceUserRepo) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	return false, errors.New("not implemented")
}

func (r *userServiceUserRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	return false, errors.New("not implemented")
}

func (r *userServiceUserRepo) Create(ctx context.Context, input repository.CreateUserInput) (repository.User, error) {
	return repository.User{}, errors.New("not implemented")
}

func (r *userServiceUserRepo) FindByUsernameOrEmail(ctx context.Context, usernameOrEmail string) (repository.User, bool, error) {
	return repository.User{}, false, errors.New("not implemented")
}

func (r *userServiceUserRepo) FindByID(ctx context.Context, id int64) (repository.User, bool, error) {
	if r.foundUser.ID == id && r.found {
		return r.foundUser, true, nil
	}
	return repository.User{}, false, nil
}

func (r *userServiceUserRepo) CountByCriteria(ctx context.Context, criteria repository.UserCriteria) (int64, error) {
	r.countCalls++
	return r.count, nil
}

func (r *userServiceUserRepo) ListUsers(ctx context.Context, criteria repository.UserCriteria, limit int32, offset int32) ([]repository.User, error) {
	return append([]repository.User(nil), r.page...), nil
}

func (r *userServiceUserRepo) UpdateStatus(ctx context.Context, id int64, status repository.UserStatus) (int64, error) {
	r.updatedStatus = status
	if r.foundUser.ID == id {
		r.foundUser.Status = status
	}
	return r.updateRows, nil
}

type userServiceRoleRepo struct {
	rolesByUserID map[int64][]string
	batchRows     []repository.UserRoleCode
	singleCalls   int
	batchCalls    int
}

func (r *userServiceRoleRepo) FindByCode(ctx context.Context, code string) (repository.Role, bool, error) {
	return repository.Role{}, false, errors.New("not implemented")
}

func (r *userServiceRoleRepo) FindRoleCodesByUserID(ctx context.Context, userID int64) ([]string, error) {
	r.singleCalls++
	return append([]string(nil), r.rolesByUserID[userID]...), nil
}

func (r *userServiceRoleRepo) FindRoleCodesByUserIDs(ctx context.Context, userIDs []int64) ([]repository.UserRoleCode, error) {
	r.batchCalls++
	return append([]repository.UserRoleCode(nil), r.batchRows...), nil
}

func (r *userServiceRoleRepo) AddRoleToUser(ctx context.Context, userID, roleID int64) (int64, error) {
	return 0, errors.New("not implemented")
}
