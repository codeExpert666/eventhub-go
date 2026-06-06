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
