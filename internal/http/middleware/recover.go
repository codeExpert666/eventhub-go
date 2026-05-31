package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/requestid"
	"eventhub-go/internal/http/response"
	platformlog "eventhub-go/internal/platform/log"
)

// Recover 返回一个 HTTP 中间件，用于把请求处理链中的 panic 转换为统一的内部错误响应。
//
// 它承担三件事：
//  1. 捕获后续 middleware 或 handler 在当前请求 goroutine 中抛出的 panic；
//  2. 记录 panic 内容、调用栈和 request id，方便服务端定位问题；
//  3. 在响应尚未提交时返回统一错误码 COMMON-500，避免泄露 panic 细节。
//
// 通常应在 RequestID 之后注册，这样 panic 日志和错误响应都能带上同一个 request id。
// 该中间件只能捕获当前请求 goroutine 内的 panic；如果业务代码另起 goroutine，
// 那个 goroutine 需要自行 recover 并上报错误。
func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	// 允许调用方传 nil，避免测试或最小化启动场景因为没有显式传 logger 而 panic。
	if logger == nil {
		logger = slog.Default()
	}

	return func(next http.Handler) http.Handler {
		// http.HandlerFunc 将普通函数适配为 http.Handler，使它可以插入标准中间件链。
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorder := &recoverResponseWriter{ResponseWriter: w}
			// defer 必须在调用 next.ServeHTTP 之前注册，才能覆盖后续处理链中的 panic。
			// recover 只有在 defer 函数中调用才会生效；没有 panic 时它会返回 nil。
			defer func() {
				if recovered := recover(); recovered != nil {
					id := requestid.FromContext(r.Context())
					// panic 的值可以是任意类型，fmt.Sprint 能把常见值稳定转成日志字符串。
					// debug.Stack 只写入服务端日志，不返回给客户端，避免暴露内部实现细节。
					platformlog.WithRequestID(logger, id).ErrorContext(
						r.Context(),
						"panic recovered from http request",
						"panic", fmt.Sprint(recovered),
						"stack", string(debug.Stack()),
						"responseCommitted", recorder.Committed(),
					)
					// 如果响应头或响应体已经提交，HTTP 层无法再安全改写状态码和响应体。
					// 这种情况下只记录日志，避免在已有响应后追加 COMMON-500，导致客户端收到损坏 JSON。
					if recorder.Committed() {
						return
					}
					// 未提交响应时，对外返回统一内部错误；真实 panic 内容只出现在服务端日志中。
					response.WriteError(recorder, r, apperror.New(apperror.CommonInternal, ""))
				}
			}()

			next.ServeHTTP(recorder, r)
		})
	}
}

// recoverResponseWriter 记录响应是否已经提交，帮助 recover 判断还能不能写统一错误体。
//
// 它不改变正常写出行为，只在 WriteHeader 或 Write 发生时标记 committed。
type recoverResponseWriter struct {
	http.ResponseWriter
	committed bool
}

func (w *recoverResponseWriter) WriteHeader(status int) {
	if w.committed {
		return
	}
	w.committed = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *recoverResponseWriter) Write(data []byte) (int, error) {
	if !w.committed {
		w.committed = true
	}
	return w.ResponseWriter.Write(data)
}

func (w *recoverResponseWriter) Committed() bool {
	return w.committed
}

func (w *recoverResponseWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
