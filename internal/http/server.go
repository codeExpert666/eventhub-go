package http

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"eventhub-go/internal/config"
)

// Server 管理进程级 HTTP 服务的生命周期。
//
// 它在内部持有标准库的 *http.Server，但不把底层对象直接暴露给 main 包。
// 这样应用启动和关闭 HTTP 服务时，只需要依赖 NewServer 和 Run 这两个入口。
type Server struct {
	httpServer *http.Server
	logger     *slog.Logger
}

// NewServer 创建 EventHub 进程使用的 HTTP 服务。
//
// 这里集中完成服务装配：监听地址来自配置，Handler 由 NewRouter 创建并负责路由和中间件，
// 超时参数在 http.Server 字段旁说明，便于理解每个配置影响的是哪个阶段。
func NewServer(cfg config.Config, logger *slog.Logger, options ...RouterOption) *Server {
	return &Server{
		httpServer: &http.Server{
			Addr:    cfg.Addr(),
			Handler: NewRouter(cfg, logger, options...),
			// ReadHeaderTimeout 限制的是“读取请求头”的最长时间，不是整个请求的处理时间。
			// 例如客户端已经建立 TCP 连接，但迟迟不把 HTTP 方法、路径、Header 等信息发完，
			// 超过 5 秒后服务端就会主动结束这次连接，避免慢速连接长期占用服务资源。
			ReadHeaderTimeout: 5 * time.Second,
		},
		logger: logger,
	}
}

// Run 启动 HTTP 服务，并阻塞等待服务退出或 ctx 被取消。
//
// 调用方传入可取消的 context 后，就可以通过取消 ctx 来触发进程退出流程。
// 当 ctx 被取消时，Run 会尝试优雅关闭：正在处理的请求最多有 10 秒完成，
// 新连接会被 http.Server.Shutdown 拒绝。
func (s *Server) Run(ctx context.Context) error {
	// ListenAndServe 通常会一直阻塞，放到 goroutine 中后，Run 才能同时等待 ctx 取消。
	// errCh 只会接收一次最终错误；容量为 1 可以避免接收方因 Shutdown 错误提前返回时，
	// goroutine 卡在发送结果上，形成 goroutine 泄露。
	errCh := make(chan error, 1)
	go func() {
		s.logger.Info("eventhub http server starting", "addr", s.httpServer.Addr)
		errCh <- s.httpServer.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		// 这里不能复用已经取消的 ctx；Shutdown 需要一个新的、带超时的 context，
		// 用来给正在处理的请求最多 10 秒完成收尾。
		// 超时后 Shutdown 会返回 context deadline exceeded；它不会主动中断仍在执行的 handler，
		// 当前 Run 会把这个错误交给上层处理。
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
			return err
		}
		err := <-errCh
		// Shutdown 成功后，ListenAndServe 会返回 http.ErrServerClosed。
		// 这是标准库约定的正常停止信号，不应该当作业务失败继续向外返回。
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}
