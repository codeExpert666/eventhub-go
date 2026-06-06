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
	platformredis "eventhub-go/internal/platform/redis"

	goredis "github.com/redis/go-redis/v9"
)

// PlatformDeps 聚合进程级平台依赖。
type PlatformDeps struct {
	Config   config.Config
	Logger   *slog.Logger
	Clock    clock.Clock
	Database *sql.DB
	Redis    *goredis.Client
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
	if cfg.Database.DSN != "" {
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
	}

	if cfg.Redis.Addr != "" {
		client := platformredis.NewClient(platformredis.Config{
			Addr:         cfg.Redis.Addr,
			Username:     cfg.Redis.Username,
			Password:     cfg.Redis.Password,
			DB:           cfg.Redis.DB,
			DialTimeout:  cfg.Redis.DialTimeout,
			ReadTimeout:  cfg.Redis.ReadTimeout,
			WriteTimeout: cfg.Redis.WriteTimeout,
		})
		if err := platformredis.Ping(ctx, client); err != nil {
			_ = client.Close()
			if deps.Database != nil {
				_ = deps.Database.Close()
			}
			return PlatformDeps{}, err
		}
		deps.Redis = client
	}

	return deps, nil
}
