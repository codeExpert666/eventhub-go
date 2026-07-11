package config

import (
	"testing"

	openapispec "eventhub-go/api/openapi"
)

func TestLoadOpenAPIDefaultsByEnv(t *testing.T) {
	tests := []struct {
		name                     string
		env                      string
		enabled                  bool
		requestValidationEnabled bool
	}{
		{name: "dev defaults docs and request validation enabled", env: EnvDev, enabled: true, requestValidationEnabled: true},
		{name: "test defaults docs and request validation enabled", env: EnvTest, enabled: true, requestValidationEnabled: true},
		{name: "prod defaults docs disabled and request validation enabled", env: EnvProd, enabled: false, requestValidationEnabled: true},
		{name: "unknown env falls back to dev defaults", env: "local", enabled: true, requestValidationEnabled: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("EVENTHUB_ENV", tt.env)
			t.Setenv("OPENAPI_ENABLED", "")
			t.Setenv("OPENAPI_REQUEST_VALIDATION_ENABLED", "")
			t.Setenv("OPENAPI_SPEC_PATH", "")

			cfg := Load()

			if cfg.OpenAPI.Enabled != tt.enabled {
				t.Fatalf("OpenAPI enabled: got %t want %t", cfg.OpenAPI.Enabled, tt.enabled)
			}
			if cfg.OpenAPI.RequestValidationEnabled != tt.requestValidationEnabled {
				t.Fatalf("OpenAPI request validation enabled: got %t want %t", cfg.OpenAPI.RequestValidationEnabled, tt.requestValidationEnabled)
			}
			if cfg.OpenAPI.AssetRoot != openapispec.AssetRoot {
				t.Fatalf("OpenAPI asset root: got %q want %q", cfg.OpenAPI.AssetRoot, openapispec.AssetRoot)
			}
			if cfg.OpenAPI.SpecPath != openapispec.DefaultSpecPath {
				t.Fatalf("OpenAPI spec path: got %q want %q", cfg.OpenAPI.SpecPath, openapispec.DefaultSpecPath)
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

func TestLoadOpenAPIAssetRootCanBeOverridden(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want string
	}{
		{name: "custom absolute path", env: "/app/api/openapi", want: "/app/api/openapi"},
		{name: "blank value falls back to default", env: "   ", want: openapispec.AssetRoot},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OPENAPI_ASSET_ROOT", tt.env)

			cfg := Load()

			if cfg.OpenAPI.AssetRoot != tt.want {
				t.Fatalf("OpenAPI asset root: got %q want %q", cfg.OpenAPI.AssetRoot, tt.want)
			}
		})
	}
}

func TestLoadOpenAPIRequestValidationEnabledCanBeOverridden(t *testing.T) {
	tests := []struct {
		name    string
		env     string
		value   string
		enabled bool
	}{
		{name: "prod can be explicitly disabled as break glass", env: EnvProd, value: "false", enabled: false},
		{name: "dev can be explicitly disabled", env: EnvDev, value: "false", enabled: false},
		{name: "invalid value falls back to enabled default", env: EnvProd, value: "maybe", enabled: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("EVENTHUB_ENV", tt.env)
			t.Setenv("OPENAPI_REQUEST_VALIDATION_ENABLED", tt.value)

			cfg := Load()

			if cfg.OpenAPI.RequestValidationEnabled != tt.enabled {
				t.Fatalf("OpenAPI request validation enabled: got %t want %t", cfg.OpenAPI.RequestValidationEnabled, tt.enabled)
			}
		})
	}
}

func TestLoadOpenAPISpecPathCanBeOverridden(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want string
	}{
		{name: "custom absolute path", env: "/app/api/openapi/eventhub.yaml", want: "/app/api/openapi/eventhub.yaml"},
		{name: "blank value falls back to default", env: "   ", want: openapispec.DefaultSpecPath},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OPENAPI_SPEC_PATH", tt.env)

			cfg := Load()

			if cfg.OpenAPI.SpecPath != tt.want {
				t.Fatalf("OpenAPI spec path: got %q want %q", cfg.OpenAPI.SpecPath, tt.want)
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
