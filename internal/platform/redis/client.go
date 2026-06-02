package redis

import (
	"context"
	"errors"

	goredis "github.com/redis/go-redis/v9"
)

const defaultAddr = "localhost:6379"

var ErrMissingClient = errors.New("redis client is nil")

// NewClient 创建 Redis 客户端。当前只提供连接底座，不参与认证强一致。
func NewClient(cfg Config) *goredis.Client {
	addr := cfg.Addr
	if addr == "" {
		addr = defaultAddr
	}
	return goredis.NewClient(&goredis.Options{
		Addr:         addr,
		Username:     cfg.Username,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})
}

// Ping 验证 Redis 客户端连通性。
func Ping(ctx context.Context, client *goredis.Client) error {
	if client == nil {
		return ErrMissingClient
	}
	return client.Ping(ctx).Err()
}
