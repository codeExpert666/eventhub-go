package auth

import (
	"context"

	"eventhub-go/internal/repository"
)

// Refresh 使用旧 refresh token 轮换新的 token pair。
func (s *Service) Refresh(ctx context.Context, command RefreshCommand) (RefreshResult, error) {
	oldRefreshToken, err := s.refreshToken.Parse(command.RefreshToken)
	if err != nil {
		return RefreshResult{}, invalidRefreshTokenError()
	}
	oldRefreshTokenHash, err := s.refreshToken.Hash(oldRefreshToken)
	if err != nil {
		return RefreshResult{}, invalidRefreshTokenError()
	}

	refreshedAt := s.clock.Now()
	var result RefreshResult
	if err := s.transactor.WithinTx(ctx, func(txCtx context.Context) error {
		session, found, err := s.sessions.FindByRefreshTokenHash(txCtx, oldRefreshTokenHash)
		if err != nil {
			return err
		}
		if !found || session.Status != repository.AuthSessionStatusActive || !session.RefreshExpiresAt.After(refreshedAt) {
			return invalidRefreshTokenError()
		}

		foundUser, found, err := s.users.FindByID(txCtx, session.UserID)
		if err != nil {
			return err
		}
		if !found || foundUser.Status != repository.UserStatusEnabled {
			return invalidRefreshTokenError()
		}
		userInfo, err := s.userService.GetByID(txCtx, foundUser.ID)
		if err != nil {
			return err
		}

		newRefreshToken, err := s.refreshToken.Generate()
		if err != nil {
			return err
		}
		newRefreshTokenHash, err := s.refreshToken.Hash(newRefreshToken)
		if err != nil {
			return err
		}
		refreshTTL := s.refreshToken.RefreshTokenTTL()
		rows, err := s.sessions.ConditionalRotate(txCtx, repository.ConditionalRotateAuthSessionInput{
			SessionID:           session.SessionID,
			OldRefreshTokenHash: oldRefreshTokenHash,
			OldVersion:          session.Version,
			NewRefreshTokenHash: newRefreshTokenHash,
			RefreshedAt:         refreshedAt,
			RefreshExpiresAt:    refreshedAt.Add(refreshTTL),
		})
		if err != nil {
			return err
		}
		if rows != 1 {
			return invalidRefreshTokenError()
		}

		accessToken, err := s.tokens.IssueAccessToken(foundUser.ID, session.SessionID, s.accessTTL)
		if err != nil {
			return err
		}
		result = RefreshResult{
			AccessToken:         accessToken,
			RefreshToken:        newRefreshToken,
			AuthorizationScheme: authorizationSchemeBearer,
			ExpiresIn:           int64(s.accessTTL.Seconds()),
			RefreshExpiresIn:    int64(refreshTTL.Seconds()),
			SessionID:           session.SessionID,
			User:                userInfo,
		}
		return nil
	}); err != nil {
		return RefreshResult{}, err
	}
	return result, nil
}
