// Package system 提供系统级应用服务用例。
package system

import (
	"context"
	"time"

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

// PingResult 表示 system ping 端点的服务层结果。
type PingResult struct {
	ServiceName    string
	ActiveProfiles []string
	ServerTime     time.Time
}

// EchoCommand 表示回显已校验消息的服务层输入。
type EchoCommand struct {
	Message string
	Tag     *string
}

// EchoResult 表示 echo 端点的服务层结果。
type EchoResult struct {
	Message  string
	Tag      *string
	EchoedAt time.Time
}

// HealthResult 表示 actuator health 端点的服务层结果。
type HealthResult struct {
	Status string
}

// InfoResult 表示 actuator info 端点的服务层结果。
type InfoResult struct {
	App     AppInfo
	Runtime RuntimeInfo
}

// AppInfo 描述应用身份信息和激活 profile 状态。
type AppInfo struct {
	Name           string
	Env            string
	Version        string
	ActiveProfiles []string
}

// RuntimeInfo 描述当前进程产生的运行时数据。
type RuntimeInfo struct {
	ServerTime time.Time
}

// Ping 返回基础服务存活数据。
func (s *Service) Ping(ctx context.Context) PingResult {
	return PingResult{
		ServiceName:    s.cfg.AppName,
		ActiveProfiles: s.cfg.ActiveProfiles(),
		ServerTime:     s.clock.Now(),
	}
}

// Echo 返回已校验消息和服务端时间戳。
func (s *Service) Echo(ctx context.Context, command EchoCommand) EchoResult {
	return EchoResult{
		Message:  command.Message,
		Tag:      command.Tag,
		EchoedAt: s.clock.Now(),
	}
}

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
