// Package config 负责加载 EventHub 进程启动所需的运行时配置。
//
// 当前实现只从环境变量读取配置，并为本地开发提供保守默认值。
// 这让应用在没有额外配置文件的情况下也能启动，同时保留容器化部署时
// 通过环境变量覆盖配置的能力。
package config

import (
	"log/slog"
	"strconv"
)

const (
	defaultAppName = "eventhub-backend"
	defaultPort    = 8080
	defaultVersion = "dev"
)

// Config 聚合应用启动时需要传递给下游组件的配置。
//
// main 函数在进程启动时调用 Load 得到一份 Config，随后将它传给日志、
// HTTP server 等组件。这样配置来源集中在本包中，下游代码只依赖明确的
// 结构化字段，而不需要到处直接读取环境变量。
type Config struct {
	// AppName 是日志、监控或健康检查中展示的应用名称。
	AppName string

	// Env 是标准化后的运行环境，只会是 dev、test 或 prod。
	Env string

	// Port 是 HTTP 服务监听端口。非法或非正数环境变量会回退到默认端口。
	Port int

	// Version 表示当前构建版本，默认值 dev 适合本地运行。
	Version string

	// Log 保存日志相关配置，后续可以继续扩展格式、输出位置等字段。
	Log LogConfig
}

// LogConfig 保存日志系统的启动配置。
type LogConfig struct {
	// Level 控制 slog 输出的最低日志级别。
	// Go 标准库 slog 内置常用等级从低到高依次是：
	// DEBUG、INFO、WARN、ERROR。级别越高，日志越重要；设置为 INFO 时，
	// DEBUG 日志会被过滤，INFO/WARN/ERROR 会正常输出。
	Level slog.Level
}

// Load 从环境变量加载配置，并对外部输入做标准化和兜底处理。
//
// 本函数不会因为配置缺失或格式错误直接退出进程；对于当前阶段的基础配置，
// 更适合回退到默认值，保证本地开发和测试环境有较低启动成本。
func Load() Config {
	cfg := Config{
		AppName: getEnv("EVENTHUB_APP_NAME", defaultAppName),
		Env:     normalizeEnv(getEnv("EVENTHUB_ENV", EnvDev)),
		Port:    getEnvInt("EVENTHUB_HTTP_PORT", defaultPort),
		Version: getEnv("EVENTHUB_VERSION", defaultVersion),
		Log: LogConfig{
			Level: parseLogLevel(getEnv("EVENTHUB_LOG_LEVEL", "INFO")),
		},
	}

	// 生产环境避免开启 DEBUG 日志，降低敏感信息泄露和日志量失控的风险。
	if cfg.Env == EnvProd && cfg.Log.Level < slog.LevelInfo {
		cfg.Log.Level = slog.LevelInfo
	}
	return cfg
}

// Addr 返回 net/http 期望的监听地址格式。
//
// 只返回 ":端口" 表示监听所有可用网卡；如果未来需要绑定特定 host，
// 可以在 Config 中增加 Host 字段并在这里统一拼接。
func (c Config) Addr() string {
	return ":" + strconv.Itoa(c.Port)
}
