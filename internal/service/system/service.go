// Package system 提供系统级应用服务用例。
package system

import (
	"eventhub-go/internal/config"
	"eventhub-go/internal/platform/clock"
)

// Service 在 HTTP 传输层之外组装系统端点数据。
type Service struct {
	cfg   config.Config
	clock clock.Clock
}

// NewService 创建系统服务。
func NewService(cfg config.Config, clk clock.Clock) *Service {
	if clk == nil {
		clk = clock.RealClock{}
	}
	return &Service{cfg: cfg, clock: clk}
}
