package providers

import (
	"eventhub-go/internal/config"
	systemhandler "eventhub-go/internal/http/handler/system"
	"eventhub-go/internal/platform/clock"
	systemsvc "eventhub-go/internal/service/system"
)

// SystemDeps 聚合 system 模块装配结果。
type SystemDeps struct {
	Service *systemsvc.Service
	Handler *systemhandler.Handler
}

// ProviderSystem 创建 system service 和 handler。
func ProviderSystem(cfg config.Config, clk clock.Clock) SystemDeps {
	service := systemsvc.NewService(cfg, clk)
	return SystemDeps{
		Service: service,
		Handler: systemhandler.NewHandler(service),
	}
}
