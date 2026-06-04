package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

// Run 完成应用装配、启动 HTTP 服务，并阻塞直到服务退出。
//
// 收到中断或终止信号时，Run 会通过 context 通知 HTTP 服务执行优雅关闭。
// http.ErrServerClosed 表示服务已按预期关闭；非预期主错误只补充阶段上下文后返回，
// 由 main 统一记录，避免同一个错误在生命周期层和进程入口重复打印。
func Run() error {
	// NotifyContext 基于 Background 创建可取消的进程生命周期 context。
	// 当进程收到 Ctrl+C 或 SIGTERM 时，ctx.Done() 会关闭，HTTP server 随后进入优雅关闭流程。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	go func() {
		<-ctx.Done()
		// 第一次退出信号到达后停止信号通知注册，恢复后续信号的默认行为。
		// 如果优雅关闭卡住，用户再次发送退出信号时可以直接终止进程。
		stop()
	}()
	// 服务正常退出或启动失败时也主动释放 signal.NotifyContext 注册的资源。
	defer stop()

	application, err := Bootstrap(ctx)
	if err != nil {
		return err
	}
	defer func() {
		if err := application.Close(); err != nil {
			application.logger.Error("failed to close application resources", "error", err)
		}
	}()

	// Run 会阻塞直到 HTTP 服务停止。优雅关闭返回的 http.ErrServerClosed 不视为错误。
	if err := application.server.Run(ctx); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("run http server: %w", err)
	}
	return nil
}
