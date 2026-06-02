// Package db 提供跨业务的数据库连接、事务和错误识别能力。
package db

import "time"

// defaultDriverName 是当前仓库默认使用的数据库驱动名称。
const defaultDriverName = "mysql"

// Config 保存 MySQL 连接池配置。
type Config struct {
	// DSN 是传递给数据库驱动的连接字符串。
	DSN string
	// MaxOpenConns 限制连接池同时打开的最大连接数。
	MaxOpenConns int
	// MaxIdleConns 限制连接池保留的最大空闲连接数。
	MaxIdleConns int
	// ConnMaxLifetime 限制单个连接可被复用的最长时间。
	ConnMaxLifetime time.Duration
	// ConnMaxIdleTime 限制空闲连接在连接池中保留的最长时间。
	ConnMaxIdleTime time.Duration
}
