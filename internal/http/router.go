package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/config"
	"eventhub-go/internal/http/handler"
	"eventhub-go/internal/http/middleware"
	"eventhub-go/internal/http/response"
)

// NewRouter 组装应用的 HTTP 路由树，并返回可直接挂载到 http.Server 的 Handler。
//
// Router 层只负责三类事情：
//  1. 注册全局中间件，例如 requestId 注入、panic 恢复；
//  2. 建立 URL、HTTP 方法和具体 handler 方法之间的映射；
//  3. 统一未匹配路由和不支持方法的错误响应。
//
// 具体业务规则应继续放在 handler/service/repository 等更内层模块中，避免路由层直接承载业务判断。
func NewRouter(cfg config.Config, logger *slog.Logger) http.Handler {
	// chi.NewRouter 创建一棵空路由树。全局中间件需要先注册，再注册具体路由，
	// 这样后续所有端点都会经过同一套请求追踪和异常保护逻辑。
	router := chi.NewRouter()
	router.Use(middleware.RequestID(logger))
	router.Use(middleware.Recover(logger))

	// SystemHandler 只暴露系统探活、信息查询和基础回显接口。
	// 路由层持有 handler 实例，但不关心 handler 内部如何读取配置或组织响应。
	systemHandler := handler.NewSystemHandler(cfg)

	// /api/v1 前缀用于业务 API，保持版本化入口，便于后续在不破坏旧客户端的前提下演进契约。
	router.Get("/api/v1/system/ping", systemHandler.Ping)
	router.Post("/api/v1/system/echo", systemHandler.Echo)

	// /actuator/* 保留 Spring Boot Actuator 风格的运维端点命名，方便和 Java 版部署、监控习惯对齐。
	router.Get("/actuator/health", systemHandler.Health)
	router.Head("/actuator/health", systemHandler.HealthHead)
	router.Get("/actuator/info", systemHandler.Info)
	router.Head("/actuator/info", systemHandler.InfoHead)

	// chi 会区分“路径不存在”和“路径存在但 HTTP 方法不支持”。
	// 当前项目统一映射为 COMMON-404，保证对外错误响应格式稳定，并避免暴露额外的路由细节。
	router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		response.WriteError(w, r, apperror.New(apperror.CommonNotFound, "请求的资源不存在"))
	})
	router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		response.WriteError(w, r, apperror.New(apperror.CommonNotFound, "请求的资源不存在"))
	})

	return router
}
