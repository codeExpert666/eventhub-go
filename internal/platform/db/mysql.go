package db

import (
	"context"
	"database/sql"
	"errors"

	drivermysql "github.com/go-sql-driver/mysql"
)

// database/sql 提供通用连接池和访问接口；go-sql-driver/mysql 提供 MySQL 驱动注册、DSN 解析和协议实现。
// 本文件只处理 MySQL 专属 DSN 配置，其余连接生命周期交给 database/sql 管理。

// ErrMissingDSN 表示数据库连接字符串未配置。
var ErrMissingDSN = errors.New("database dsn is required")

// OpenMySQL 打开 MySQL 连接池并执行一次 ping，保证返回的连接池可用。
func OpenMySQL(ctx context.Context, cfg Config) (*sql.DB, error) {
	if cfg.DSN == "" {
		return nil, ErrMissingDSN
	}

	dsn, err := normalizeMySQLDSN(cfg.DSN)
	if err != nil {
		return nil, err
	}

	// sql.Open 返回的是连接池句柄，不会立刻建立网络连接。
	database, err := sql.Open(defaultDriverName, dsn)
	if err != nil {
		return nil, err
	}

	// 仅在配置值大于 0 时覆盖 database/sql 的默认连接池策略。
	if cfg.MaxOpenConns > 0 {
		database.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		database.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		database.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		database.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}

	// 初始化阶段主动探活，避免把不可用连接池交给上层业务。
	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		return nil, err
	}
	return database, nil
}

// normalizeMySQLDSN 规范化 MySQL DSN，并让时间列可扫描为 time.Time。
func normalizeMySQLDSN(dsn string) (string, error) {
	driverConfig, err := drivermysql.ParseDSN(dsn)
	if err != nil {
		return "", err
	}
	driverConfig.ParseTime = true
	return driverConfig.FormatDSN(), nil
}
