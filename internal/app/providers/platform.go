// Package providers 放置应用 composition root 内部的依赖装配辅助。
package providers

import (
	"context"
	"database/sql"
	"log/slog"

	"eventhub-go/internal/config"
	"eventhub-go/internal/platform/clock"
	platformdb "eventhub-go/internal/platform/db"
	platformlog "eventhub-go/internal/platform/log"
)

// PlatformDeps 聚合进程级平台依赖。
type PlatformDeps struct {
	Config   config.Config
	Logger   *slog.Logger
	Clock    clock.Clock
	Database *sql.DB
}

// ProviderPlatform 加载配置并创建进程级平台依赖。
func ProviderPlatform(ctx context.Context) (PlatformDeps, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	cfg := config.Load()
	logger := platformlog.New(cfg)
	slog.SetDefault(logger)

	deps := PlatformDeps{
		Config: cfg,
		Logger: logger,
		Clock:  clock.RealClock{},
	}
	if cfg.Database.DSN == "" {
		return deps, nil
	}

	database, err := platformdb.OpenMySQL(ctx, platformdb.Config{
		DSN:             cfg.Database.DSN,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
		ConnMaxIdleTime: cfg.Database.ConnMaxIdleTime,
	})
	if err != nil {
		return PlatformDeps{}, err
	}
	deps.Database = database
	return deps, nil
}
