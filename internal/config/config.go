// Package config 负责加载 EventHub 进程启动所需的运行时配置。
//
// 当前实现只从环境变量读取配置，并为本地开发提供保守默认值。
// 这让应用在没有额外配置文件的情况下也能启动，同时保留容器化部署时
// 通过环境变量覆盖配置的能力。
package config

import (
	"log/slog"
	"strconv"
	"time"

	openapispec "eventhub-go/api/openapi"
)

const (
	defaultAppName                  = "eventhub-backend"
	defaultPort                     = 8080
	defaultVersion                  = "dev"
	defaultAccessTokenTTL           = 30 * time.Minute
	defaultRefreshTokenTTL          = 30 * 24 * time.Hour
	defaultAccessTokenSigningSecret = "eventhub-dev-access-token-secret-for-local-development"
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

	// Database 保存 MySQL 连接池配置。DSN 为空时，运行态不会装配依赖数据库的业务路由。
	Database DatabaseConfig

	// Redis 保存 Redis 连接配置。Addr 为空时，运行态不会创建 Redis 客户端。
	Redis RedisConfig

	// AuthToken 保存 access token 和 refresh token 的签发配置。
	AuthToken AuthTokenConfig

	// OpenAPI 保存接口文档入口配置。生产环境默认关闭，避免暴露接口契约。
	OpenAPI OpenAPIConfig
}

// LogConfig 保存日志系统的启动配置。
type LogConfig struct {
	// Level 控制 slog 输出的最低日志级别。
	// Go 标准库 slog 内置常用等级从低到高依次是：
	// DEBUG、INFO、WARN、ERROR。级别越高，日志越重要；设置为 INFO 时，
	// DEBUG 日志会被过滤，INFO/WARN/ERROR 会正常输出。
	Level slog.Level
}

// DatabaseConfig 保存 MySQL 连接配置。
type DatabaseConfig struct {
	// DSN 是 MySQL 连接字符串；为空时表示当前进程不装配数据库依赖。
	DSN string
	// MaxOpenConns 限制连接池同时打开的最大连接数，避免压垮数据库。
	MaxOpenConns int
	// MaxIdleConns 限制连接池保留的空闲连接数，平衡复用效率和资源占用。
	MaxIdleConns int
	// ConnMaxLifetime 控制单个连接的最长存活时间，用于定期回收老连接。
	ConnMaxLifetime time.Duration
	// ConnMaxIdleTime 控制连接在空闲状态下可保留的最长时间。
	ConnMaxIdleTime time.Duration
}

// RedisConfig 保存 Redis 连接配置。
type RedisConfig struct {
	// Addr 是 Redis 服务地址，例如 localhost:6379 或 redis:6379。
	Addr string
	// Username 是 Redis ACL 用户名；本地开发通常为空。
	Username string
	// Password 是 Redis 密码；本地开发通常为空。
	Password string
	// DB 是 Redis 逻辑库编号。
	DB int
	// DialTimeout 控制建立 Redis 连接的最长等待时间。
	DialTimeout time.Duration
	// ReadTimeout 控制 Redis 读操作的最长等待时间。
	ReadTimeout time.Duration
	// WriteTimeout 控制 Redis 写操作的最长等待时间。
	WriteTimeout time.Duration
}

// AuthTokenConfig 保存认证令牌配置。
type AuthTokenConfig struct {
	// Issuer 写入 token 的 iss claim，用于标识签发方。
	Issuer string
	// AccessTokenSigningSecret 是 access token 签名密钥；生产环境必须显式配置。
	AccessTokenSigningSecret string
	// AccessTokenTTL 控制 access token 的有效期。
	AccessTokenTTL time.Duration
	// RefreshTokenTTL 控制 refresh token 的有效期。
	RefreshTokenTTL time.Duration
}

// OpenAPIConfig 保存 OpenAPI 文档入口配置。
type OpenAPIConfig struct {
	// Enabled 控制是否注册 /openapi.yaml 和 /swagger/* 文档入口。
	Enabled bool
	// AssetRoot 指向 OpenAPI YAML 与 Swagger UI 静态资源所在的本地目录；相对路径按进程当前工作目录解析。
	AssetRoot string
}

// Load 从环境变量加载配置，并对外部输入做标准化和兜底处理。
//
// 本函数不会因为配置缺失或格式错误直接退出进程；对于当前阶段的基础配置，
// 更适合回退到默认值，保证本地开发和测试环境有较低启动成本。
func Load() Config {
	env := normalizeEnv(getEnv("EVENTHUB_ENV", EnvDev))

	// 开发和测试环境允许使用内置默认密钥，降低本地启动成本；
	// 生产环境必须显式配置签名密钥，避免误用可预测的开发密钥签发 token。
	signingSecretFallback := defaultAccessTokenSigningSecret
	if env == EnvProd {
		signingSecretFallback = ""
	}

	cfg := Config{
		AppName: getEnv("EVENTHUB_APP_NAME", defaultAppName),
		Env:     env,
		Port:    getEnvInt("EVENTHUB_HTTP_PORT", defaultPort),
		Version: getEnv("EVENTHUB_VERSION", defaultVersion),
		Log: LogConfig{
			Level: parseLogLevel(getEnv("EVENTHUB_LOG_LEVEL", "INFO")),
		},
		Database: DatabaseConfig{
			DSN:             getEnv("EVENTHUB_MYSQL_DSN", ""),
			MaxOpenConns:    getEnvInt("EVENTHUB_MYSQL_MAX_OPEN_CONNS", 10),
			MaxIdleConns:    getEnvInt("EVENTHUB_MYSQL_MAX_IDLE_CONNS", 2),
			ConnMaxLifetime: getEnvDuration("EVENTHUB_MYSQL_CONN_MAX_LIFETIME", 30*time.Minute),
			ConnMaxIdleTime: getEnvDuration("EVENTHUB_MYSQL_CONN_MAX_IDLE_TIME", 5*time.Minute),
		},
		Redis: RedisConfig{
			Addr:         getEnv("EVENTHUB_REDIS_ADDR", ""),
			Username:     getEnv("EVENTHUB_REDIS_USERNAME", ""),
			Password:     getEnv("EVENTHUB_REDIS_PASSWORD", ""),
			DB:           getEnvInt("EVENTHUB_REDIS_DB", 0),
			DialTimeout:  getEnvDuration("EVENTHUB_REDIS_DIAL_TIMEOUT", 5*time.Second),
			ReadTimeout:  getEnvDuration("EVENTHUB_REDIS_READ_TIMEOUT", 3*time.Second),
			WriteTimeout: getEnvDuration("EVENTHUB_REDIS_WRITE_TIMEOUT", 3*time.Second),
		},
		AuthToken: AuthTokenConfig{
			Issuer:                   getEnv("EVENTHUB_AUTH_TOKEN_ISSUER", defaultAppName),
			AccessTokenSigningSecret: getEnv("EVENTHUB_ACCESS_TOKEN_SIGNING_SECRET", signingSecretFallback),
			AccessTokenTTL:           getEnvDuration("EVENTHUB_ACCESS_TOKEN_TTL", defaultAccessTokenTTL),
			RefreshTokenTTL:          getEnvDuration("EVENTHUB_REFRESH_TOKEN_TTL", defaultRefreshTokenTTL),
		},
		OpenAPI: OpenAPIConfig{
			Enabled:   getEnvBool("OPENAPI_ENABLED", defaultOpenAPIEnabled(env)),
			AssetRoot: getEnv("OPENAPI_ASSET_ROOT", openapispec.AssetRoot),
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
