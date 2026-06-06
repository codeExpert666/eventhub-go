package providers

import (
	"net/http"

	apphttp "eventhub-go/internal/http"
	openapihandler "eventhub-go/internal/http/handler/openapi"
)

// HTTPDeps 聚合 HTTP router 和 server 装配结果。
type HTTPDeps struct {
	Router http.Handler
	Server *apphttp.Server
}

// ProviderHTTP 创建应用 router 和 HTTP server。
func ProviderHTTP(platform PlatformDeps, system SystemDeps, auth AuthDeps, user UserDeps) HTTPDeps {
	var openAPI *openapihandler.OpenAPIHandler
	if platform.Config.OpenAPI.Enabled {
		openAPI = openapihandler.NewOpenAPIHandler()
	}
	routerDeps := apphttp.RouterDependencies{
		System:         system.Handler,
		Auth:           auth.Handler,
		User:           user.Handler,
		OpenAPI:        openAPI,
		AuthMiddleware: auth.Middleware,
	}
	router := apphttp.NewRouter(platform.Logger, routerDeps)
	return HTTPDeps{
		Router: router,
		Server: apphttp.NewServer(platform.Config, platform.Logger, router),
	}
}
