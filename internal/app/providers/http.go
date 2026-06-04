package providers

import (
	"net/http"

	apphttp "eventhub-go/internal/http"
)

// HTTPDeps 聚合 HTTP router 和 server 装配结果。
type HTTPDeps struct {
	Router http.Handler
	Server *apphttp.Server
}

// ProviderHTTP 创建应用 router 和 HTTP server。
func ProviderHTTP(platform PlatformDeps, system SystemDeps, auth AuthDeps, user UserDeps) HTTPDeps {
	routerDeps := apphttp.RouterDependencies{
		System:         system.Handler,
		Auth:           auth.Handler,
		User:           user.Handler,
		AuthMiddleware: auth.Middleware,
	}
	router := apphttp.NewRouter(platform.Logger, routerDeps)
	return HTTPDeps{
		Router: router,
		Server: apphttp.NewServer(platform.Config, platform.Logger, router),
	}
}
