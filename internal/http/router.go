package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"eventhub-go/internal/apperror"
	authhandler "eventhub-go/internal/http/handler/auth"
	openapihandler "eventhub-go/internal/http/handler/openapi"
	systemhandler "eventhub-go/internal/http/handler/system"
	userhandler "eventhub-go/internal/http/handler/user"
	"eventhub-go/internal/http/middleware"
	"eventhub-go/internal/http/response"
)

// RouterDependencies 是 router 注册路由所需的显式依赖。
type RouterDependencies struct {
	System  *systemhandler.Handler
	Auth    *authhandler.Handler
	User    *userhandler.Handler
	OpenAPI *openapihandler.OpenAPIHandler
	// RequestContract 在 generated strict wrapper 之前执行 OpenAPI request contract gate。
	RequestContract func(http.Handler) http.Handler
}

// NewRouter 组装应用的 HTTP 路由树，并返回可直接挂载到 http.Server 的 Handler。
//
// Router 层只负责三类事情：
//  1. 注册全局中间件，例如 requestId 注入、panic 恢复；
//  2. 建立 URL、HTTP 方法和具体 handler 方法之间的映射；
//  3. 统一未匹配路由和不支持方法的错误响应。
//
// 具体业务规则应继续放在 handler/service/repository 等更内层模块中，避免路由层直接承载业务判断。
func NewRouter(logger *slog.Logger, deps RouterDependencies) http.Handler {
	// chi.NewRouter 创建一棵空路由树。全局中间件需要先注册，再注册具体路由，
	// 这样后续所有端点都会经过同一套请求追踪和异常保护逻辑。
	router := chi.NewRouter()
	router.Use(middleware.RequestID(logger))
	router.Use(middleware.Recover(logger))

	// OpenAPI routes 是业务 API 与 actuator API 的运行时入口。
	// router 主流程只接入 generated chi wrapper；具体的 strict-server 适配、认证编排和错误映射
	// 收敛在 openapi_routes.go，避免这里重新维护 eventhub.yaml 已声明的 path/method 细节。
	registerOpenAPIRoutes(router, deps)

	// OpenAPI / Swagger 文档入口受 OPENAPI_ENABLED 控制，不属于 strict-server 的业务 API 契约。
	// Provider 未传入 OpenAPI handler 时不注册这些路由，请求会继续落入统一 NotFound，
	// 保持 prod 默认隐藏文档入口并返回 COMMON-404 的行为。
	if deps.OpenAPI != nil {
		router.Get("/openapi.yaml", deps.OpenAPI.YAML)
		router.Get("/swagger", deps.OpenAPI.RedirectSwagger)
		router.Get("/swagger/", deps.OpenAPI.SwaggerUI)
		router.Get("/swagger/index.html", deps.OpenAPI.SwaggerUI)
		router.Get("/swagger/*", deps.OpenAPI.SwaggerAsset)
	}

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
