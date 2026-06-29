// Package app 负责 EventHub 进程级应用装配。
package app

import (
	"context"
	"fmt"

	"eventhub-go/internal/app/providers"
)

// Bootstrap 加载运行时配置并完成基础组件装配。
// 这里只命名 err 返回值，用于 defer 在后续装配失败时清理已创建资源。
func Bootstrap(ctx context.Context) (_ *Application, err error) {
	// 启动期 ctx 只传给可能执行阻塞 I/O 或探活的 provider。
	// 当前 ProviderPlatform 会通过 MySQL PingContext 响应启动取消；后续纯对象装配
	// provider 不应接收或保存启动 ctx。
	platform, err := providers.ProviderPlatform(ctx)
	if err != nil {
		return nil, fmt.Errorf("provide platform dependencies: %w", err)
	}
	defer func() {
		if err != nil {
			if platform.Redis != nil {
				if closeErr := platform.Redis.Close(); closeErr != nil {
					platform.Logger.Error("failed to close bootstrap redis after setup error", "error", closeErr)
				}
			}
			if platform.Database != nil {
				if closeErr := platform.Database.Close(); closeErr != nil {
					platform.Logger.Error("failed to close bootstrap database after setup error", "error", closeErr)
				}
			}
		}
	}()

	system := providers.ProviderSystem(platform.Config, platform.Clock)
	user := providers.ProviderUser(platform.Database)
	auth, err := providers.ProviderAuth(platform, user)
	if err != nil {
		return nil, fmt.Errorf("provide auth dependencies: %w", err)
	}
	httpDeps, err := providers.ProviderHTTP(platform, system, auth, user)
	if err != nil {
		return nil, fmt.Errorf("provide http dependencies: %w", err)
	}

	return NewApplication(platform.Logger, httpDeps.Server, platform.Database, platform.Redis), nil
}
