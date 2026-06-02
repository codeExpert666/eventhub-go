// Package db 提供跨业务的数据库连接、事务和错误识别能力。
package db

import "time"

const defaultDriverName = "mysql"

// Config 保存 MySQL 连接池配置。
type Config struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}
