package providers

import (
	"errors"

	authhandler "eventhub-go/internal/http/handler/auth"
	"eventhub-go/internal/http/middleware"
	platformdb "eventhub-go/internal/platform/db"
	repositorymysql "eventhub-go/internal/repository/mysql"
	"eventhub-go/internal/security/jwt"
	"eventhub-go/internal/security/password"
	"eventhub-go/internal/security/refresh"
	authsvc "eventhub-go/internal/service/auth"
)

// AuthDeps 聚合 auth 模块装配结果。
type AuthDeps struct {
	Service    *authsvc.Service
	Handler    *authhandler.Handler
	Middleware *middleware.AuthMiddleware
}

// ProviderAuth 在数据库可用时创建 auth service、handler 和 middleware。
func ProviderAuth(platform PlatformDeps, user UserDeps) (AuthDeps, error) {
	if platform.Database == nil {
		if platform.Logger != nil {
			platform.Logger.Warn("mysql dsn is not configured; auth routes are not registered")
		}
		return AuthDeps{}, nil
	}
	if user.Users == nil || user.Roles == nil || user.Service == nil {
		return AuthDeps{}, errors.New("user dependencies are required to enable auth")
	}

	jwtCodec, err := jwt.NewCodec(jwt.Config{
		Issuer:        platform.Config.AuthToken.Issuer,
		SigningSecret: platform.Config.AuthToken.AccessTokenSigningSecret,
		AccessTTL:     platform.Config.AuthToken.AccessTokenTTL,
	})
	if err != nil {
		return AuthDeps{}, err
	}

	sessionRepo := repositorymysql.NewAuthSessionRepository(platform.Database)
	authService := authsvc.NewService(
		user.Users,
		user.Roles,
		sessionRepo,
		platformdb.NewTransactor(platform.Database, nil),
		password.NewBCryptHasher(),
		jwtCodec,
		refresh.NewManager(platform.Config.AuthToken.RefreshTokenTTL),
		user.Service,
		platform.Clock,
	)

	return AuthDeps{
		Service:    authService,
		Handler:    authhandler.NewHandler(authService),
		Middleware: middleware.NewAuth(jwtCodec, user.Service),
	}, nil
}
