package auth

import (
	"context"
	"errors"
	"strings"

	"eventhub-go/internal/apperror"
	platformdb "eventhub-go/internal/platform/db"
	"eventhub-go/internal/repository"
	usersvc "eventhub-go/internal/service/user"
)

const defaultUserRoleCode = "USER"

// Register 创建普通用户并绑定默认 USER 角色。
func (s *Service) Register(ctx context.Context, command RegisterCommand) (usersvc.UserResult, error) {
	username := strings.TrimSpace(command.Username)
	email := strings.ToLower(strings.TrimSpace(command.Email))

	var created repository.User
	if err := s.transactor.WithinTx(ctx, func(txCtx context.Context) error {
		usernameExists, err := s.users.ExistsByUsername(txCtx, username)
		if err != nil {
			return err
		}
		if usernameExists {
			return duplicateUsernameError()
		}
		emailExists, err := s.users.ExistsByEmail(txCtx, email)
		if err != nil {
			return err
		}
		if emailExists {
			return duplicateEmailError()
		}

		passwordHash, err := s.passwords.Hash(command.Password)
		if err != nil {
			return err
		}
		created, err = s.users.Create(txCtx, repository.CreateUserInput{
			Username:     username,
			Email:        email,
			PasswordHash: passwordHash,
			Status:       repository.UserStatusEnabled,
		})
		if err != nil {
			if platformdb.IsUniqueConstraintError(err) {
				return duplicateAccountError()
			}
			return err
		}

		userRole, found, err := s.roles.FindByCode(txCtx, defaultUserRoleCode)
		if err != nil {
			return err
		}
		if !found {
			return errors.New("default USER role is missing")
		}
		rows, err := s.roles.AddRoleToUser(txCtx, created.ID, userRole.ID)
		if err != nil {
			return err
		}
		if rows != 1 {
			return errors.New("failed to bind default USER role")
		}
		return nil
	}); err != nil {
		var appErr *apperror.AppError
		if errors.As(err, &appErr) {
			return usersvc.UserResult{}, appErr
		}
		return usersvc.UserResult{}, err
	}

	return s.userService.GetByID(ctx, created.ID)
}
