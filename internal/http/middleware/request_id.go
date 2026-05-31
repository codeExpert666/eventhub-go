package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"eventhub-go/internal/platform/idgen"
	platformlog "eventhub-go/internal/platform/log"
)

// RequestID 返回一个 HTTP 中间件，用于为每个请求建立稳定的 request id。
//
// request id 会同时写入三处：
//  1. 响应头 X-Request-Id，方便客户端或网关拿到本次请求标识；
//  2. request.Context，方便后续 handler/response/recover 等模块读取；
//  3. 访问日志字段，方便根据同一个 ID 串起一次请求的处理过程。
//
// 如果客户端已经传入合法的 X-Request-Id，则复用该值；否则重新生成一个安全的新 ID。
//
// 从设计模式看，这个中间件函数本身是 Decorator 模式：
// 它接收一个已有的 http.Handler，返回一个增加了 request id 处理和访问日志能力的新 http.Handler。
func RequestID(logger *slog.Logger) func(http.Handler) http.Handler {
	// 允许调用方传 nil，避免测试或最小化启动场景因为没有显式传 logger 而 panic。
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		// http.HandlerFunc 是 Go 标准库提供的适配器模式：
		// 它把一个普通函数 func(ResponseWriter, *Request) 适配成实现了 ServeHTTP 方法的 http.Handler。
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// 先尝试复用上游传入的 request id，便于跨服务或跨网关追踪。
			// 复用前必须校验格式，避免把不可控字符串写入日志、响应头或上下文。
			id := r.Header.Get(idgen.HeaderRequestID)
			if !idgen.ValidRequestID(id) {
				id = idgen.NewRequestID()
			}

			// 无论 request id 来自客户端还是服务端生成，都显式回写响应头，
			// 这样调用方可以稳定地知道服务端最终采用的是哪个 ID。
			w.Header().Set(idgen.HeaderRequestID, id)

			startedAt := time.Now()
			// statusRecorder 包装原始 ResponseWriter，用于在不影响 handler 写响应的前提下记录状态码和响应体大小。
			// net/http 在 handler 未显式调用 WriteHeader 时，会在首次 Write 时默认返回 200 OK；
			// 因此这里先把 status 初始化为 http.StatusOK，避免正常响应在日志中被记录成 0。
			recorder := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			// 将 request id 放入新的 context，再把新请求对象传给后续处理链。
			// WithContext 只浅拷贝 Request 并替换 Context，不会修改原请求；这里不改 Header/URL/Form 等字段，
			// 因此无需使用会复制更多字段的 Clone。
			ctx := idgen.WithRequestID(r.Context(), id)
			next.ServeHTTP(recorder, r.WithContext(ctx))

			// 只有在后续 handler 执行完毕后，才能拿到最终状态码、响应字节数和耗时。
			// requestId 字段由 WithRequestID 显式写入；InfoContext 传入 ctx，是为了让日志跟随请求上下文，
			// 并为后续接入 trace 或 context-aware 日志处理器保留入口。
			platformlog.WithRequestID(logger, id).InfoContext(
				ctx,
				"http request completed",
				"method", r.Method,
				"path", r.URL.Path,
				"status", recorder.status,
				"bytes", recorder.bytes,
				"durationMs", time.Since(startedAt).Milliseconds(),
			)
		})
	}
}

// statusRecorder 透传 ResponseWriter 的真实写入行为，同时记录本次响应的状态码和字节数。
//
// 它只服务于访问日志统计，不负责改变响应内容或错误映射。
//
// 从设计模式看，statusRecorder 是 Decorator 模式：
// 它嵌入原始的 http.ResponseWriter，并只增强 WriteHeader 和 Write，其余能力继续交给原始 ResponseWriter。
type statusRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

// WriteHeader 记录 handler 显式设置的 HTTP 状态码。
//
// net/http 约定响应头只能写一次；如果业务代码重复调用 WriteHeader，这里保持和标准库一致，
// 只记录并透传第一次状态码。
func (r *statusRecorder) WriteHeader(status int) {
	if r.wroteHeader {
		return
	}
	r.status = status
	r.wroteHeader = true
	r.ResponseWriter.WriteHeader(status)
}

// Write 透传响应体写入，并累计实际写出的字节数，用于请求完成日志。
func (r *statusRecorder) Write(data []byte) (int, error) {
	written, err := r.ResponseWriter.Write(data)
	r.bytes += written
	return written, err
}
