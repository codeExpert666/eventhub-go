// Package app 负责 EventHub 进程级应用装配。
package app

import (
	"context"
	"database/sql"
	"log/slog"

	"eventhub-go/internal/config"
	apphttp "eventhub-go/internal/http"
	authhandler "eventhub-go/internal/http/handler/auth"
	systemhandler "eventhub-go/internal/http/handler/system"
	userhandler "eventhub-go/internal/http/handler/user"
	"eventhub-go/internal/http/middleware"
	"eventhub-go/internal/platform/clock"
	platformdb "eventhub-go/internal/platform/db"
	platformlog "eventhub-go/internal/platform/log"
	repositorymysql "eventhub-go/internal/repository/mysql"
	"eventhub-go/internal/security/jwt"
	"eventhub-go/internal/security/password"
	"eventhub-go/internal/security/refresh"
	authsvc "eventhub-go/internal/service/auth"
	systemsvc "eventhub-go/internal/service/system"
	usersvc "eventhub-go/internal/service/user"
)

// Application 聚合进程生命周期内共享的基础组件。
//
// app 包是 composition root（依赖装配入口），只负责把配置、日志和 HTTP server
// 等进程级组件组装起来；业务规则继续放在 service/domain/repository 等更内层 package。
type Application struct {
	// logger 是进程级共享日志器，用于启动、关闭和基础设施层面的运行日志。
	logger *slog.Logger
	// server 封装 HTTP 路由、中间件和底层 http.Server 生命周期。
	server *apphttp.Server
	// database 是进程级 MySQL 连接池；未配置 DSN 时为空。
	database *sql.DB
}

// Bootstrap 加载运行时配置并完成基础组件装配。
func Bootstrap() (*Application, error) {
	// 配置在进程启动时加载一次，后续组件共享同一份配置快照。
	cfg := config.Load()

	// 日志器依赖配置初始化，并设置为 slog 默认 logger，保证各层日志格式与级别一致。
	logger := platformlog.New(cfg)
	slog.SetDefault(logger)

	routerDependencies, database, err := buildRouterDependencies(context.Background(), cfg, logger)
	if err != nil {
		return nil, err
	}
	router := apphttp.NewRouter(logger, routerDependencies)

	// app 包只做进程级依赖装配，HTTP 细节继续由 internal/http 封装。
	return &Application{
		logger:   logger,
		server:   apphttp.NewServer(cfg, logger, router),
		database: database,
	}, nil
}

// Close 释放应用持有的进程级资源。
func (a *Application) Close() error {
	if a == nil || a.database == nil {
		return nil
	}
	return a.database.Close()
}

func buildRouterDependencies(
	ctx context.Context,
	cfg config.Config,
	logger *slog.Logger,
) (apphttp.RouterDependencies, *sql.DB, error) {
	systemService := systemsvc.NewService(cfg, clock.RealClock{})
	deps := apphttp.RouterDependencies{
		System: systemhandler.NewHandler(systemService),
	}

	if cfg.Database.DSN == "" {
		logger.Warn("mysql dsn is not configured; auth routes are not registered")
		return deps, nil, nil
	}

	database, err := platformdb.OpenMySQL(ctx, platformdb.Config{
		DSN:             cfg.Database.DSN,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.Database.ConnMaxIdleTime,
	})
	if err != nil {
		return apphttp.RouterDependencies{}, nil, err
	}

	jwtCodec, err := jwt.NewCodec(jwt.Config{
		Issuer:        cfg.AuthToken.Issuer,
		SigningSecret: cfg.AuthToken.AccessTokenSigningSecret,
		AccessTTL:     cfg.AuthToken.AccessTokenTTL,
	})
	if err != nil {
		_ = database.Close()
		return apphttp.RouterDependencies{}, nil, err
	}

	userRepo := repositorymysql.NewUserRepository(database)
	roleRepo := repositorymysql.NewRoleRepository(database)
	sessionRepo := repositorymysql.NewAuthSessionRepository(database)
	transactor := platformdb.NewTransactor(database, nil)
	userService := usersvc.NewService(userRepo, roleRepo)
	authService := authsvc.NewService(authsvc.Dependencies{
		Users:        userRepo,
		Roles:        roleRepo,
		Sessions:     sessionRepo,
		Transactor:   transactor,
		Passwords:    password.NewBCryptHasher(),
		Tokens:       jwtCodec,
		RefreshToken: refresh.NewManager(cfg.AuthToken.RefreshTokenTTL),
		UserService:  userService,
	})
	authMiddleware := middleware.NewAuth(jwtCodec, userService)

	deps.Auth = authhandler.NewHandler(authService)
	deps.User = userhandler.NewHandler(userService)
	deps.AuthMiddleware = authMiddleware

	return deps, database, nil
}
