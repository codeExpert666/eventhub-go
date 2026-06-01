package system

import "context"

// Health 返回当前最小健康状态。
func (s *Service) Health(ctx context.Context) HealthResult {
	return HealthResult{Status: "UP"}
}

// Info 返回应用信息和运行时元数据。
func (s *Service) Info(ctx context.Context) InfoResult {
	return InfoResult{
		App: AppInfo{
			Name:           s.cfg.AppName,
			Env:            s.cfg.Env,
			Version:        s.cfg.Version,
			ActiveProfiles: s.cfg.ActiveProfiles(),
		},
		Runtime: RuntimeInfo{ServerTime: s.clock.Now()},
	}
}
