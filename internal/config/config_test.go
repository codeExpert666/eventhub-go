package config

import "testing"

func TestLoadOpenAPIDefaultsByEnv(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		enabled bool
	}{
		{name: "dev defaults enabled", env: EnvDev, enabled: true},
		{name: "test defaults enabled", env: EnvTest, enabled: true},
		{name: "prod defaults disabled", env: EnvProd, enabled: false},
		{name: "unknown env falls back to dev enabled", env: "local", enabled: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("EVENTHUB_ENV", tt.env)
			t.Setenv("OPENAPI_ENABLED", "")

			cfg := Load()

			if cfg.OpenAPI.Enabled != tt.enabled {
				t.Fatalf("OpenAPI enabled: got %t want %t", cfg.OpenAPI.Enabled, tt.enabled)
			}
		})
	}
}

func TestLoadOpenAPIEnabledCanBeOverridden(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		value   string
		enabled bool
	}{
		{name: "prod can be explicitly enabled", env: EnvProd, value: "true", enabled: true},
		{name: "dev can be explicitly disabled", env: EnvDev, value: "false", enabled: false},
		{name: "invalid value falls back to env default", env: EnvProd, value: "maybe", enabled: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("EVENTHUB_ENV", tt.env)
			t.Setenv("OPENAPI_ENABLED", tt.value)

			cfg := Load()

			if cfg.OpenAPI.Enabled != tt.enabled {
				t.Fatalf("OpenAPI enabled: got %t want %t", cfg.OpenAPI.Enabled, tt.enabled)
			}
		})
	}
}

func TestLoadRedisConfig(t *testing.T) {
	t.Setenv("EVENTHUB_REDIS_ADDR", "redis:6379")
	t.Setenv("EVENTHUB_REDIS_USERNAME", "eventhub")
	t.Setenv("EVENTHUB_REDIS_PASSWORD", "secret")
	t.Setenv("EVENTHUB_REDIS_DB", "2")
	t.Setenv("EVENTHUB_REDIS_DIAL_TIMEOUT", "2s")
	t.Setenv("EVENTHUB_REDIS_READ_TIMEOUT", "1500ms")
	t.Setenv("EVENTHUB_REDIS_WRITE_TIMEOUT", "2500ms")

	cfg := Load()

	if cfg.Redis.Addr != "redis:6379" {
		t.Fatalf("redis addr: got %q", cfg.Redis.Addr)
	}
	if cfg.Redis.Username != "eventhub" {
		t.Fatalf("redis username: got %q", cfg.Redis.Username)
	}
	if cfg.Redis.Password != "secret" {
		t.Fatalf("redis password: got %q", cfg.Redis.Password)
	}
	if cfg.Redis.DB != 2 {
		t.Fatalf("redis db: got %d", cfg.Redis.DB)
	}
	if cfg.Redis.DialTimeout.String() != "2s" {
		t.Fatalf("redis dial timeout: got %s", cfg.Redis.DialTimeout)
	}
	if cfg.Redis.ReadTimeout.String() != "1.5s" {
		t.Fatalf("redis read timeout: got %s", cfg.Redis.ReadTimeout)
	}
	if cfg.Redis.WriteTimeout.String() != "2.5s" {
		t.Fatalf("redis write timeout: got %s", cfg.Redis.WriteTimeout)
	}
}
