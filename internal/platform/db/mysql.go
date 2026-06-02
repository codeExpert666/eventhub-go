package db

import (
	"context"
	"database/sql"
	"errors"

	drivermysql "github.com/go-sql-driver/mysql"
)

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

	database, err := sql.Open(defaultDriverName, dsn)
	if err != nil {
		return nil, err
	}
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

	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		return nil, err
	}
	return database, nil
}

func normalizeMySQLDSN(dsn string) (string, error) {
	driverConfig, err := drivermysql.ParseDSN(dsn)
	if err != nil {
		return "", err
	}
	driverConfig.ParseTime = true
	return driverConfig.FormatDSN(), nil
}
