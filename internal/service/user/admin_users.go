package user

import (
	"context"
	"math"
	"strings"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/page"
	"eventhub-go/internal/repository"
)

// ListUsers 按管理员筛选条件分页查询用户摘要。
func (s *Service) ListUsers(ctx context.Context, query AdminUserListQuery) (page.Response[UserResult], error) {
	pageRequest, err := pageRequestFromQuery(query)
	if err != nil {
		return page.Response[UserResult]{}, err
	}
	offset, err := pageOffset(pageRequest)
	if err != nil {
		return page.Response[UserResult]{}, err
	}
	status, err := statusFromFilter(query.Status)
	if err != nil {
		return page.Response[UserResult]{}, err
	}
	criteria := repository.UserCriteria{
		Username:      normalizeFilter(query.Username),
		Email:         normalizeEmailFilter(query.Email),
		Status:        status,
		CreatedAtFrom: query.CreatedAtFrom,
		CreatedAtTo:   query.CreatedAtTo,
		UpdatedAtFrom: query.UpdatedAtFrom,
		UpdatedAtTo:   query.UpdatedAtTo,
	}

	total, err := s.users.CountByCriteria(ctx, criteria)
	if err != nil {
		return page.Response[UserResult]{}, err
	}
	if total == 0 {
		return page.NewResponse([]UserResult{}, pageRequest, total)
	}

	users, err := s.users.ListUsers(ctx, criteria, int32(pageRequest.Size), offset)
	if err != nil {
		return page.Response[UserResult]{}, err
	}
	rolesByUserID, err := s.findRolesByUserIDs(ctx, users)
	if err != nil {
		return page.Response[UserResult]{}, err
	}
	return page.NewResponse(toUserResults(users, rolesByUserID), pageRequest, total)
}

// UpdateStatus 更新用户状态，并返回更新后的用户摘要。
func (s *Service) UpdateStatus(ctx context.Context, command UpdateUserStatusCommand) (UserResult, error) {
	if command.UserID <= 0 {
		return UserResult{}, validationError("userId", "userId 必须是正整数")
	}
	status, err := statusFromRequired(command.Status)
	if err != nil {
		return UserResult{}, err
	}

	var result UserResult
	update := func(txCtx context.Context) error {
		rows, err := s.users.UpdateStatus(txCtx, command.UserID, status)
		if err != nil {
			return err
		}
		if rows == 0 {
			return apperror.New(apperror.CommonNotFound, "用户不存在")
		}
		result, err = s.GetByID(txCtx, command.UserID)
		return err
	}
	if s.transactor == nil {
		if err := update(ctx); err != nil {
			return UserResult{}, err
		}
		return result, nil
	}
	if err := s.transactor.WithinTx(ctx, update); err != nil {
		return UserResult{}, err
	}
	return result, nil
}

func pageRequestFromQuery(query AdminUserListQuery) (page.Request, error) {
	pageNumber := query.Page
	if pageNumber == 0 {
		pageNumber = page.DefaultPage
	}
	size := query.Size
	if size == 0 {
		size = page.DefaultSize
	}
	request, err := page.NewRequest(pageNumber, size)
	if err != nil {
		return page.Request{}, validationError("page", err.Error())
	}
	return request, nil
}

func pageOffset(request page.Request) (int32, error) {
	if request.Page < 1 || request.Size < 1 {
		return 0, validationError("page", "page 或 size 不合法")
	}
	pageIndex := int64(request.Page - 1)
	size := int64(request.Size)
	if pageIndex > int64(math.MaxInt32)/size {
		return 0, validationError("page", "page 过大")
	}
	return int32(pageIndex * size), nil
}

func statusFromFilter(value string) (*repository.UserStatus, error) {
	normalized := strings.TrimSpace(value)
	if normalized == "" {
		return nil, nil
	}
	status, err := statusFromRequired(normalized)
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func statusFromRequired(value string) (repository.UserStatus, error) {
	switch strings.TrimSpace(value) {
	case string(repository.UserStatusEnabled):
		return repository.UserStatusEnabled, nil
	case string(repository.UserStatusDisabled):
		return repository.UserStatusDisabled, nil
	default:
		return "", validationError("status", "用户状态只能是 ENABLED 或 DISABLED")
	}
}

func (s *Service) findRolesByUserIDs(ctx context.Context, users []repository.User) (map[int64][]string, error) {
	if len(users) == 0 {
		return map[int64][]string{}, nil
	}
	userIDs := make([]int64, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}
	rows, err := s.roles.FindRoleCodesByUserIDs(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	rolesByUserID := make(map[int64][]string, len(users))
	for _, row := range rows {
		rolesByUserID[row.UserID] = append(rolesByUserID[row.UserID], row.RoleCode)
	}
	return rolesByUserID, nil
}

func toUserResults(users []repository.User, rolesByUserID map[int64][]string) []UserResult {
	results := make([]UserResult, 0, len(users))
	for _, user := range users {
		results = append(results, toUserResult(user, rolesByUserID[user.ID]))
	}
	return results
}

func normalizeFilter(value string) string {
	return strings.TrimSpace(value)
}

func normalizeEmailFilter(value string) string {
	return strings.ToLower(normalizeFilter(value))
}

func validationError(field, message string) *apperror.AppError {
	return apperror.WithDetails(
		apperror.CommonValidation,
		"请求参数校验失败",
		apperror.Details{field: message},
	)
}
