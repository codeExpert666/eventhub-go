// Package app 负责 EventHub 进程级应用装配。
package app

import (
	"database/sql"
	"log/slog"

	apphttp "eventhub-go/internal/http"
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

// NewApplication 创建进程级应用对象。
func NewApplication(logger *slog.Logger, server *apphttp.Server, database *sql.DB) *Application {
	return &Application{
		logger:   logger,
		server:   server,
		database: database,
	}
}

// Close 释放应用持有的进程级资源。
func (a *Application) Close() error {
	if a == nil || a.database == nil {
		return nil
	}
	return a.database.Close()
}
