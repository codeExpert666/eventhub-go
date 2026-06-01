package system

import "context"

// Ping 返回基础服务存活数据。
func (s *Service) Ping(ctx context.Context) PingResult {
	return PingResult{
		ServiceName:    s.cfg.AppName,
		ActiveProfiles: s.cfg.ActiveProfiles(),
		ServerTime:     s.clock.Now(),
	}
}
