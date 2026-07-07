package providers

import (
	"fmt"
	"net/http"

	apphttp "eventhub-go/internal/http"
	"eventhub-go/internal/http/contract"
	openapihandler "eventhub-go/internal/http/handler/openapi"
)

// HTTPDeps 聚合 HTTP router 和 server 装配结果。
type HTTPDeps struct {
	Router          http.Handler
	Server          *apphttp.Server
	RequestContract *contract.Spec
}

// ProviderHTTP 创建应用 router 和 HTTP server。
func ProviderHTTP(platform PlatformDeps, system SystemDeps, auth AuthDeps, user UserDeps) (HTTPDeps, error) {
	var openAPI *openapihandler.OpenAPIHandler
	if platform.Config.OpenAPI.Enabled {
		var err error
		openAPI, err = openapihandler.NewOpenAPIHandler(platform.Config.OpenAPI.AssetRoot)
		if err != nil {
			return HTTPDeps{}, fmt.Errorf("initialize openapi handler: %w", err)
		}
	}
	var requestContract *contract.Spec
	var requestContractMiddleware func(http.Handler) http.Handler
	if platform.Config.OpenAPI.RequestValidationEnabled {
		var err error
		requestContract, err = contract.LoadSpec(platform.Config.OpenAPI.SpecPath)
		if err != nil {
			return HTTPDeps{}, fmt.Errorf("initialize openapi request contract: %w", err)
		}
		requestValidator, err := contract.NewRequestValidator(requestContract)
		if err != nil {
			return HTTPDeps{}, fmt.Errorf("initialize openapi request validator: %w", err)
		}
		requestContractMiddleware = requestValidator.Middleware
	}
	routerDeps := apphttp.RouterDependencies{
		System:          system.Handler,
		Auth:            auth.Handler,
		User:            user.Handler,
		OpenAPI:         openAPI,
		Authenticate:    auth.Authenticate,
		RequestContract: requestContractMiddleware,
	}
	router := apphttp.NewRouter(platform.Logger, routerDeps)
	return HTTPDeps{
		Router:          router,
		Server:          apphttp.NewServer(platform.Config, platform.Logger, router),
		RequestContract: requestContract,
	}, nil
}
