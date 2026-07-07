package config

import "strings"

const (
	// EnvDev 表示本地开发环境，是未知或未配置环境名的兜底值。
	EnvDev = "dev"
	// EnvTest 表示测试环境，通常用于自动化测试或 CI。
	EnvTest = "test"
	// EnvProd 表示生产环境，会启用更保守的运行时约束。
	EnvProd = "prod"
)

// ActiveProfiles 返回当前激活的运行环境列表。
//
// 这个命名对齐 Java/Spring 中 profile 的概念，便于迁移时表达
// “当前环境上下文”。Go 端目前只支持单一环境。
func (c Config) ActiveProfiles() []string {
	if c.Env == "" {
		return []string{EnvDev}
	}
	return []string{c.Env}
}

// normalizeEnv 将外部传入的环境名收敛到应用内部支持的固定集合。
func normalizeEnv(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case EnvTest:
		return EnvTest
	case EnvProd:
		return EnvProd
	default:
		return EnvDev
	}
}

func defaultOpenAPIEnabled(env string) bool {
	return env != EnvProd
}

func defaultOpenAPIRequestValidationEnabled(env string) bool {
	return env != EnvProd
}
