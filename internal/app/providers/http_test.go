package providers

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	openapispec "eventhub-go/api/openapi"
	"eventhub-go/internal/config"
	"eventhub-go/internal/platform/clock"
)

func TestProviderHTTPRegistersOpenAPIRoutesWithoutDatabase(t *testing.T) {
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
	httpDeps, err := ProviderHTTP(platform, system, auth, user)
	if err != nil {
		t.Fatalf("provide http deps: %v", err)
	}

	assertStatus(t, httpDeps.Router, http.MethodGet, "/api/v1/system/ping", http.StatusOK)
	assertErrorCodeWithBody(
		t,
		httpDeps.Router,
		http.MethodPost,
		"/api/v1/auth/login",
		`{"usernameOrEmail":"alice","password":"Password123"}`,
		http.StatusNotFound,
		"COMMON-404",
	)
	assertErrorCode(t, httpDeps.Router, http.MethodGet, "/api/v1/me", http.StatusNotFound, "COMMON-404")
}

func TestProviderHTTPRegistersOpenAPIWhenEnabled(t *testing.T) {
	t.Parallel()

	assetRoot := writeProviderOpenAPITestAssets(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := config.Config{
		AppName: "eventhub-backend",
		Env:     config.EnvTest,
		Port:    8080,
		Version: "test",
		Log:     config.LogConfig{Level: slog.LevelError},
		OpenAPI: config.OpenAPIConfig{
			Enabled:   true,
			AssetRoot: assetRoot,
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
	httpDeps, err := ProviderHTTP(platform, system, auth, user)
	if err != nil {
		t.Fatalf("provide http deps: %v", err)
	}

	assertStatus(t, httpDeps.Router, http.MethodGet, "/openapi.yaml", http.StatusOK)
	assertBody(t, httpDeps.Router, http.MethodGet, "/openapi.yaml", providerTestOpenAPIYAML)
	assertStatus(t, httpDeps.Router, http.MethodGet, "/swagger/", http.StatusOK)
	assertContentType(t, httpDeps.Router, http.MethodGet, "/swagger/swagger-ui.css", http.StatusOK, "text/css")
	assertBody(t, httpDeps.Router, http.MethodGet, "/swagger/swagger-ui.css", providerTestSwaggerCSS)
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
	httpDeps, err := ProviderHTTP(platform, system, auth, user)
	if err != nil {
		t.Fatalf("provide http deps: %v", err)
	}

	assertErrorCode(t, httpDeps.Router, http.MethodGet, "/openapi.yaml", http.StatusNotFound, "COMMON-404")
	for _, path := range []string{
		"/swagger/",
		"/swagger/index.html",
		"/swagger/swagger-ui.css",
		"/swagger/swagger-ui-bundle.js",
	} {
		assertErrorCode(t, httpDeps.Router, http.MethodGet, path, http.StatusNotFound, "COMMON-404")
	}
}

func TestProviderHTTPRejectsMissingOpenAPIAssetsWhenEnabled(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := config.Config{
		AppName: "eventhub-backend",
		Env:     config.EnvTest,
		Port:    8080,
		Version: "test",
		Log:     config.LogConfig{Level: slog.LevelError},
		OpenAPI: config.OpenAPIConfig{
			Enabled:   true,
			AssetRoot: filepath.Join(t.TempDir(), "missing-openapi-assets"),
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

	httpDeps, err := ProviderHTTP(platform, system, auth, user)
	if err == nil {
		t.Fatal("expected provider error for missing OpenAPI assets")
	}
	if httpDeps.Router != nil || httpDeps.Server != nil {
		t.Fatalf("expected empty http deps on error, got %#v", httpDeps)
	}
	if !strings.Contains(err.Error(), "openapi asset root") ||
		!strings.Contains(err.Error(), openapispec.SpecPath) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProviderHTTPSkipsOpenAPIAssetValidationWhenDisabled(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := config.Config{
		AppName: "eventhub-backend",
		Env:     config.EnvProd,
		Port:    8080,
		Version: "prod",
		Log:     config.LogConfig{Level: slog.LevelError},
		OpenAPI: config.OpenAPIConfig{
			Enabled:   false,
			AssetRoot: filepath.Join(t.TempDir(), "missing-openapi-assets"),
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

	httpDeps, err := ProviderHTTP(platform, system, auth, user)
	if err != nil {
		t.Fatalf("provide http deps with OpenAPI disabled: %v", err)
	}
	assertErrorCode(t, httpDeps.Router, http.MethodGet, "/openapi.yaml", http.StatusNotFound, "COMMON-404")
}

func TestProviderHTTPLoadsOpenAPIContractWhenRequestValidationEnabled(t *testing.T) {
	t.Parallel()

	specPath := writeProviderContractSpec(t, providerTestValidContractYAML)
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := config.Config{
		AppName: "eventhub-backend",
		Env:     config.EnvTest,
		Port:    8080,
		Version: "test",
		Log:     config.LogConfig{Level: slog.LevelError},
		OpenAPI: config.OpenAPIConfig{
			RequestValidationEnabled: true,
			SpecPath:                 specPath,
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

	httpDeps, err := ProviderHTTP(platform, system, auth, user)
	if err != nil {
		t.Fatalf("provide http deps: %v", err)
	}
	if httpDeps.RequestContract == nil {
		t.Fatal("expected request contract spec to be loaded")
	}
	if httpDeps.RequestContract.Path != specPath {
		t.Fatalf("request contract path: got %q want %q", httpDeps.RequestContract.Path, specPath)
	}
}

func TestProviderHTTPRejectsMissingOpenAPIContractWhenRequestValidationEnabled(t *testing.T) {
	t.Parallel()

	missingPath := filepath.Join(t.TempDir(), "missing-eventhub.yaml")
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := config.Config{
		AppName: "eventhub-backend",
		Env:     config.EnvTest,
		Port:    8080,
		Version: "test",
		Log:     config.LogConfig{Level: slog.LevelError},
		OpenAPI: config.OpenAPIConfig{
			RequestValidationEnabled: true,
			SpecPath:                 missingPath,
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

	httpDeps, err := ProviderHTTP(platform, system, auth, user)
	if err == nil {
		t.Fatal("expected provider error for missing OpenAPI contract")
	}
	if httpDeps.Router != nil || httpDeps.Server != nil || httpDeps.RequestContract != nil {
		t.Fatalf("expected empty http deps on error, got %#v", httpDeps)
	}
	if !strings.Contains(err.Error(), "initialize openapi request contract") ||
		!strings.Contains(err.Error(), missingPath) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestProviderHTTPAppliesOpenAPIContractWhenRequestValidationEnabled(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := config.Config{
		AppName: "eventhub-backend",
		Env:     config.EnvTest,
		Port:    8080,
		Version: "test",
		Log:     config.LogConfig{Level: slog.LevelError},
		OpenAPI: config.OpenAPIConfig{
			RequestValidationEnabled: true,
			SpecPath:                 repoProviderOpenAPISpecPath(),
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
	httpDeps, err := ProviderHTTP(platform, system, auth, user)
	if err != nil {
		t.Fatalf("provide http deps: %v", err)
	}

	assertErrorCodeWithBodyAndContentType(
		t,
		httpDeps.Router,
		http.MethodPost,
		"/api/v1/system/echo",
		`{"message":"hello eventhub"}`,
		"text/plain",
		http.StatusBadRequest,
		"COMMON-400",
	)
}

func TestProviderHTTPSkipsOpenAPIContractWhenRequestValidationDisabled(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
	cfg := config.Config{
		AppName: "eventhub-backend",
		Env:     config.EnvTest,
		Port:    8080,
		Version: "test",
		Log:     config.LogConfig{Level: slog.LevelError},
		OpenAPI: config.OpenAPIConfig{
			RequestValidationEnabled: false,
			SpecPath:                 filepath.Join(t.TempDir(), "missing-eventhub.yaml"),
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
	httpDeps, err := ProviderHTTP(platform, system, auth, user)
	if err != nil {
		t.Fatalf("provide http deps: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/api/v1/system/echo", strings.NewReader(`{"message":"hello eventhub"}`))
	request.Header.Set("Content-Type", "text/plain")
	httpDeps.Router.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected current strict-server behavior without contract gate, got status %d body=%s", recorder.Code, recorder.Body.String())
	}
}

func assertStatus(t *testing.T, handler http.Handler, method, path string, status int) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	if recorder.Code != status {
		t.Fatalf("%s %s status: got %d want %d", method, path, recorder.Code, status)
	}
}

func assertContentType(t *testing.T, handler http.Handler, method, path string, status int, contentType string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	if recorder.Code != status {
		t.Fatalf("%s %s status: got %d want %d body=%s", method, path, recorder.Code, status, recorder.Body.String())
	}
	if got := recorder.Header().Get("Content-Type"); !strings.Contains(got, contentType) {
		t.Fatalf("%s %s content-type: got %q want containing %q", method, path, got, contentType)
	}
}

func assertBody(t *testing.T, handler http.Handler, method, path string, body string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	if got := recorder.Body.String(); got != body {
		t.Fatalf("%s %s body: got %q want %q", method, path, got, body)
	}
}

func assertErrorCode(t *testing.T, handler http.Handler, method, path string, status int, code string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))
	assertErrorRecorder(t, method, path, recorder, status, code)
}

func assertErrorCodeWithBody(t *testing.T, handler http.Handler, method, path string, requestBody string, status int, code string) {
	t.Helper()
	assertErrorCodeWithBodyAndContentType(t, handler, method, path, requestBody, "application/json", status, code)
}

func assertErrorCodeWithBodyAndContentType(t *testing.T, handler http.Handler, method, path string, requestBody string, contentType string, status int, code string) {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, strings.NewReader(requestBody))
	request.Header.Set("Content-Type", contentType)
	handler.ServeHTTP(recorder, request)
	assertErrorRecorder(t, method, path, recorder, status, code)
}

func assertErrorRecorder(t *testing.T, method, path string, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
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

const (
	providerTestOpenAPIYAML       = "openapi: 3.0.3\ninfo:\n  title: Provider Test API\n  version: test\n"
	providerTestSwaggerHTML       = "<!doctype html><title>Provider Test Swagger UI</title><div id=\"swagger-ui\"></div>\n"
	providerTestSwaggerCSS        = ".provider-swagger-ui { color: #222; }\n"
	providerTestSwaggerJS         = "window.SwaggerUIBundle = function () {};\n"
	providerTestValidContractYAML = `openapi: 3.0.3
info:
  title: Provider Contract Test API
  version: test
paths:
  /ping:
    get:
      operationId: ping
      responses:
        "200":
          description: pong
`
)

func writeProviderOpenAPITestAssets(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	swaggerDir := filepath.Join(root, filepath.FromSlash(openapispec.SwaggerDirPath))
	if err := os.MkdirAll(swaggerDir, 0o755); err != nil {
		t.Fatalf("create provider swagger asset dir: %v", err)
	}
	writeProviderTestFile(t, filepath.Join(root, filepath.FromSlash(openapispec.SpecPath)), providerTestOpenAPIYAML)
	writeProviderTestFile(t, filepath.Join(root, filepath.FromSlash(openapispec.SwaggerIndexPath)), providerTestSwaggerHTML)
	writeProviderTestFile(t, filepath.Join(root, filepath.FromSlash(openapispec.SwaggerCSSPath)), providerTestSwaggerCSS)
	writeProviderTestFile(t, filepath.Join(root, filepath.FromSlash(openapispec.SwaggerBundlePath)), providerTestSwaggerJS)
	return root
}

func writeProviderTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeProviderContractSpec(t *testing.T, content string) string {
	t.Helper()

	specPath := filepath.Join(t.TempDir(), "eventhub.yaml")
	writeProviderTestFile(t, specPath, content)
	return specPath
}

func repoProviderOpenAPISpecPath() string {
	return filepath.Join("..", "..", "..", filepath.FromSlash(openapispec.DefaultSpecPath))
}
