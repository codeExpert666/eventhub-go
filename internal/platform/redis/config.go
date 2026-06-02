// Package redis 提供跨业务 Redis 客户端创建和连通性检查能力。
package redis

import "time"

// Config 保存 Redis 连接配置。
type Config struct {
	Addr         string
	Username     string
	Password     string
	DB           int
	DialTimeout  time.Duration
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}
