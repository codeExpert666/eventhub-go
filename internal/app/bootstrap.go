// Package app 负责 EventHub 进程级应用装配。
package app

import (
	"log/slog"

	"eventhub-go/internal/config"
	apphttp "eventhub-go/internal/http"
	platformlog "eventhub-go/internal/platform/log"
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
}

// Bootstrap 加载运行时配置并完成基础组件装配。
func Bootstrap() *Application {
	// 配置在进程启动时加载一次，后续组件共享同一份配置快照。
	cfg := config.Load()

	// 日志器依赖配置初始化，并设置为 slog 默认 logger，保证各层日志格式与级别一致。
	logger := platformlog.New(cfg)
	slog.SetDefault(logger)

	// app 包只做进程级依赖装配，HTTP 细节继续由 internal/http 封装。
	return &Application{
		logger: logger,
		server: apphttp.NewServer(cfg, logger),
	}
}
