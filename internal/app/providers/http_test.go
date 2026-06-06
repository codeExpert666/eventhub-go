package providers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"eventhub-go/internal/config"
	"eventhub-go/internal/platform/clock"
)

func TestProviderHTTPRegistersOnlySystemRoutesWithoutDatabase(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := config.Config{
		AppName: "eventhub-backend",
		Env:     config.EnvTest,
		Port:    8080,
		Version: "test",
		Log:     config.LogConfig{Level: slog.LevelError},
	}
	clk := clock.RealClock{}
	platform := PlatformDeps{Config: cfg, Logger: logger, Clock: clk}
	system := ProviderSystem(cfg, clk)
	user := ProviderUser(nil)
	auth, err := ProviderAuth(platform, user)
	if err != nil {
		t.Fatalf("new auth deps: %v", err)
	}
	httpDeps := ProviderHTTP(platform, system, auth, user)

	assertStatus(t, httpDeps.Router, http.MethodGet, "/api/v1/system/ping", http.StatusOK)
	assertErrorCode(t, httpDeps.Router, http.MethodPost, "/api/v1/auth/login", http.StatusNotFound, "COMMON-404")
	assertErrorCode(t, httpDeps.Router, http.MethodGet, "/api/v1/me", http.StatusNotFound, "COMMON-404")
}

func TestProviderHTTPRegistersOpenAPIWhenEnabled(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := config.Config{
		AppName: "eventhub-backend",
		Env:     config.EnvTest,
		Port:    8080,
		Version: "test",
		Log:     config.LogConfig{Level: slog.LevelError},
		OpenAPI: config.OpenAPIConfig{
			Enabled: true,
		},
	}
	clk := clock.RealClock{}
	platform := PlatformDeps{Config: cfg, Logger: logger, Clock: clk}
	system := ProviderSystem(cfg, clk)
	user := ProviderUser(nil)
	auth, err := ProviderAuth(platform, user)
	if err != nil {
		t.Fatalf("new auth deps: %v", err)
	}
	httpDeps := ProviderHTTP(platform, system, auth, user)

	assertStatus(t, httpDeps.Router, http.MethodGet, "/openapi.yaml", http.StatusOK)
	assertStatus(t, httpDeps.Router, http.MethodGet, "/swagger/", http.StatusOK)
}

func TestProviderHTTPSkipsOpenAPIWhenDisabled(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := config.Config{
		AppName: "eventhub-backend",
		Env:     config.EnvProd,
		Port:    8080,
		Version: "prod",
		Log:     config.LogConfig{Level: slog.LevelError},
		OpenAPI: config.OpenAPIConfig{
			Enabled: false,
		},
	}
	clk := clock.RealClock{}
	platform := PlatformDeps{Config: cfg, Logger: logger, Clock: clk}
	system := ProviderSystem(cfg, clk)
	user := ProviderUser(nil)
	auth, err := ProviderAuth(platform, user)
	if err != nil {
		t.Fatalf("new auth deps: %v", err)
	}
	httpDeps := ProviderHTTP(platform, system, auth, user)

	assertErrorCode(t, httpDeps.Router, http.MethodGet, "/openapi.yaml", http.StatusNotFound, "COMMON-404")
	assertErrorCode(t, httpDeps.Router, http.MethodGet, "/swagger/index.html", http.StatusNotFound, "COMMON-404")
}

func assertStatus(t *testing.T, handler http.Handler, method, path string, status int) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	if recorder.Code != status {
		t.Fatalf("%s %s status: got %d want %d", method, path, recorder.Code, status)
	}
}

func assertErrorCode(t *testing.T, handler http.Handler, method, path string, status int, code string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	if recorder.Code != status {
		t.Fatalf("%s %s status: got %d want %d body=%s", method, path, recorder.Code, status, recorder.Body.String())
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != code {
		t.Fatalf("%s %s code: got %s want %s", method, path, body.Code, code)
	}
}
