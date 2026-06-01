package system

import "time"

// PingResult 表示 system ping 端点的服务层结果。
type PingResult struct {
	ServiceName    string
	ActiveProfiles []string
	ServerTime     time.Time
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
