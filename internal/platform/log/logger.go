// Package log 提供 EventHub 的日志初始化与日志上下文辅助方法。
//
// 项目使用 Go 标准库 slog 作为结构化日志工具。本包把日志格式、日志级别、
// 服务名、环境名等公共约定集中到一处，避免业务代码重复关心这些基础字段。
package log

import (
	"log/slog"
	"os"

	"eventhub-go/internal/config"
)

// New 根据应用配置创建项目统一使用的 slog.Logger。
//
// 当前日志会以 JSON 格式写入标准输出，这是容器化部署中最常见的做法：
// 应用只负责输出结构化日志，采集、过滤和持久化交给 Docker、Kubernetes
// 或日志平台处理。
func New(cfg config.Config) *slog.Logger {
	// Handler 决定日志写到哪里、用什么格式输出，以及最低输出级别。
	// cfg.Log.Level 来自 config.Load，会控制 DEBUG/INFO/WARN/ERROR 的过滤边界。
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: cfg.Log.Level,
	})

	// With 返回一个带默认字段的新 logger。后续每条日志都会自动携带 service 和 env，
	// 便于在日志平台中按服务名或运行环境检索。
	return slog.New(handler).With(
		"service", cfg.AppName,
		"env", cfg.Env,
	)
}

// WithRequestID 返回携带 requestId 字段的 logger。
//
// requestId 用于把一次 HTTP 请求在不同层级产生的日志串联起来。调用方拿到返回的
// logger 后继续记录日志，就能自动带上同一个 requestId。
func WithRequestID(logger *slog.Logger, requestID string) *slog.Logger {
	// 允许传入 nil，避免调用方在边界场景中因为日志器缺失而 panic。
	if logger == nil {
		logger = slog.Default()
	}
	// 没有 requestId 时直接复用原 logger，避免输出空字段造成日志噪声。
	if requestID == "" {
		return logger
	}
	return logger.With("requestId", requestID)
}
