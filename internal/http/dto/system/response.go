package system

import "time"

// PingResponse 表示 GET /api/v1/system/ping 的响应数据。
type PingResponse struct {
	ServiceName    string    `json:"serviceName"`
	ActiveProfiles []string  `json:"activeProfiles"`
	ServerTime     time.Time `json:"serverTime"`
}

// EchoResponse 表示 POST /api/v1/system/echo 的响应数据。
type EchoResponse struct {
	Message  string    `json:"message"`
	Tag      *string   `json:"tag"`
	EchoedAt time.Time `json:"echoedAt"`
}

// HealthResponse 表示 GET /actuator/health 的响应数据。
type HealthResponse struct {
	Status string `json:"status"`
}

// InfoResponse 表示 GET /actuator/info 的响应数据。
type InfoResponse struct {
	App     AppInfoResponse     `json:"app"`
	Runtime RuntimeInfoResponse `json:"runtime"`
}

// AppInfoResponse 描述 actuator info 输出中的应用元数据。
type AppInfoResponse struct {
	Name           string   `json:"name"`
	Env            string   `json:"env"`
	Version        string   `json:"version"`
	ActiveProfiles []string `json:"activeProfiles"`
}

// RuntimeInfoResponse 描述 actuator info 输出中的运行时元数据。
type RuntimeInfoResponse struct {
	ServerTime time.Time `json:"serverTime"`
}
