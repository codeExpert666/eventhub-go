// Package main 提供 EventHub HTTP 服务的可执行入口。
//
// 这个文件只负责进程启动阶段的“装配”工作：加载配置、初始化日志、
// 监听退出信号，并把控制权交给 HTTP server。业务规则、路由和中间件
// 都应继续放在 internal 包中，避免入口文件膨胀。
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"eventhub-go/internal/config"
	apphttp "eventhub-go/internal/http"
	platformlog "eventhub-go/internal/platform/log"
)

func main() {
	// 配置在进程启动时加载一次，后续组件通过同一份 cfg 保持行为一致。
	cfg := config.Load()

	// 日志器依赖配置初始化，并设置为 slog 的默认 logger。
	// 这样其他使用 slog.Default() 的代码也会沿用统一的日志格式与级别。
	logger := platformlog.New(cfg)
	slog.SetDefault(logger)

	// NotifyContext 基于 Background 创建一个可取消的 context，并监听 Ctrl+C/SIGTERM。
	// 收到这些进程退出信号时，context 会被取消；取消后 ctx.Done() 这个 channel 会关闭。
	// server.Run 监听这个 context，感知取消后再执行 HTTP server 的优雅关闭。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	// 第一次退出信号到达后立即停止 signal 通知注册，恢复后续信号的默认行为。
	// 这样如果优雅关闭卡在 10 秒窗口内，第二次 Ctrl+C 可以直接强制终止进程。
	go func() {
		<-ctx.Done()
		stop()
	}()
	// stop 会主动取消这个 context，并停止 signal 通知注册、释放相关资源。
	// stop 可以重复调用；这里保留 defer 作为服务正常退出或启动失败时的兜底清理。
	defer stop()

	// HTTP server 的路由、middleware 和底层 http.Server 由 internal/http 封装。
	// main 只负责创建和启动它，保持 handler/service/repository 的分层边界清晰。
	server := apphttp.NewServer(cfg, logger)

	// Run 会阻塞直到服务退出。http.ErrServerClosed 表示一次预期内的优雅关闭，
	// 只有非预期错误才记录日志并以非零状态码退出，方便进程管理器或 CI 识别失败。
	if err := server.Run(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("eventhub server stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}
