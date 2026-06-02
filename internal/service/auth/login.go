package auth

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"eventhub-go/internal/repository"
)

const authorizationSchemeBearer = "Bearer"

// Login 校验账号密码，创建 ACTIVE auth session，并签发 token pair。
func (s *Service) Login(ctx context.Context, command LoginCommand) (LoginResult, error) {
	usernameOrEmail := normalizeLoginIdentifier(command.UsernameOrEmail)

	var result LoginResult
	if err := s.transactor.WithinTx(ctx, func(txCtx context.Context) error {
		foundUser, found, err := s.users.FindByUsernameOrEmail(txCtx, usernameOrEmail)
		if err != nil {
			return err
		}
		if !found {
			return badCredentialsError()
		}
		matches, err := s.passwords.Matches(command.Password, foundUser.PasswordHash)
		if err != nil {
			return err
		}
		if !matches {
			return badCredentialsError()
		}
		if foundUser.Status == repository.UserStatusDisabled {
			return disabledUserError()
		}

		userInfo, err := s.userService.GetByID(txCtx, foundUser.ID)
		if err != nil {
			return err
		}
		sessionID := uuid.NewString()
		refreshToken, err := s.refreshToken.Generate()
		if err != nil {
			return err
		}
		refreshTokenHash, err := s.refreshToken.Hash(refreshToken)
		if err != nil {
			return err
		}
		issuedAt := s.clock.Now()
		refreshTTL := s.refreshToken.RefreshTokenTTL()
		lastSeenAt := issuedAt
		if _, err := s.sessions.Create(txCtx, repository.CreateAuthSessionInput{
			SessionID:        sessionID,
			UserID:           foundUser.ID,
			RefreshTokenHash: refreshTokenHash,
			Status:           repository.AuthSessionStatusActive,
			IssuedAt:         issuedAt,
			RefreshExpiresAt: issuedAt.Add(refreshTTL),
			LastSeenAt:       &lastSeenAt,
			Version:          0,
		}); err != nil {
			return err
		}
		accessToken, err := s.tokens.IssueAccessToken(foundUser.ID, sessionID)
		if err != nil {
			return err
		}

		result = LoginResult{
			AccessToken:         accessToken,
			RefreshToken:        refreshToken,
			AuthorizationScheme: authorizationSchemeBearer,
			ExpiresIn:           int64(s.tokens.AccessTokenTTL().Seconds()),
			RefreshExpiresIn:    int64(refreshTTL.Seconds()),
			SessionID:           sessionID,
			User:                userInfo,
		}
		return nil
	}); err != nil {
		return LoginResult{}, err
	}
	return result, nil
}

func normalizeLoginIdentifier(usernameOrEmail string) string {
	trimmed := strings.TrimSpace(usernameOrEmail)
	if strings.Contains(trimmed, "@") {
		return strings.ToLower(trimmed)
	}
	return trimmed
}
