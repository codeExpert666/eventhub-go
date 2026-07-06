package http_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	nethttp "net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	openapispec "eventhub-go/api/openapi"
	"eventhub-go/internal/config"
	apphttp "eventhub-go/internal/http"
	openapihandler "eventhub-go/internal/http/handler/openapi"
	systemhandler "eventhub-go/internal/http/handler/system"
	"eventhub-go/internal/http/middleware"
	"eventhub-go/internal/platform/clock"
	"eventhub-go/internal/platform/idgen"
	systemsvc "eventhub-go/internal/service/system"
)

func TestPingReturnsWrappedSuccessResponse(t *testing.T) {
	recorder := performRequest(testRouter(), nethttp.MethodGet, "/api/v1/system/ping", nil, nil)

	if recorder.Code != nethttp.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if recorder.Header().Get(idgen.HeaderRequestID) == "" {
		t.Fatal("expected request id response header")
	}

	body := decodeAPIResponse(t, recorder)
	if body["code"] != "COMMON-000" {
		t.Fatalf("unexpected code: %v", body["code"])
	}
	if body["message"] != "成功" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
	data := body["data"].(map[string]any)
	if data["serviceName"] != "eventhub-backend" {
		t.Fatalf("unexpected serviceName: %v", data["serviceName"])
	}
}

func TestEchoReturnsWrappedSuccessResponse(t *testing.T) {
	body := []byte(`{"message":"hello eventhub","tag":"bootstrap"}`)
	headers := map[string]string{"Content-Type": "application/json"}
	recorder := performRequest(testRouter(), nethttp.MethodPost, "/api/v1/system/echo", body, headers)

	if recorder.Code != nethttp.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}

	responseBody := decodeAPIResponse(t, recorder)
	data := responseBody["data"].(map[string]any)
	if data["message"] != "hello eventhub" {
		t.Fatalf("unexpected message: %v", data["message"])
	}
	if data["tag"] != "bootstrap" {
		t.Fatalf("unexpected tag: %v", data["tag"])
	}
}

func TestEchoRejectsBlankMessage(t *testing.T) {
	body := []byte(`{"message":"","tag":"bootstrap"}`)
	headers := map[string]string{"Content-Type": "application/json"}
	recorder := performRequest(testRouter(), nethttp.MethodPost, "/api/v1/system/echo", body, headers)

	if recorder.Code != nethttp.StatusBadRequest {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}

	responseBody := decodeAPIResponse(t, recorder)
	if responseBody["code"] != "COMMON-400" {
		t.Fatalf("unexpected code: %v", responseBody["code"])
	}
	if responseBody["message"] != "请求体参数校验失败" {
		t.Fatalf("unexpected message: %v", responseBody["message"])
	}
	data := responseBody["data"].(map[string]any)
	if data["message"] != "message 不能为空" {
		t.Fatalf("unexpected field error: %v", data["message"])
	}
}

func TestEchoRejectsMalformedJSON(t *testing.T) {
	body := []byte(`{"message":"hello"`)
	headers := map[string]string{"Content-Type": "application/json"}
	recorder := performRequest(testRouter(), nethttp.MethodPost, "/api/v1/system/echo", body, headers)

	if recorder.Code != nethttp.StatusBadRequest {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}

	responseBody := decodeAPIResponse(t, recorder)
	if responseBody["code"] != "COMMON-400" {
		t.Fatalf("unexpected code: %v", responseBody["code"])
	}
	if responseBody["message"] != "请求体格式不合法" {
		t.Fatalf("unexpected message: %v", responseBody["message"])
	}
	data := responseBody["data"].(map[string]any)
	if data["body"] != "请求体缺失或 JSON 格式错误" {
		t.Fatalf("unexpected body error: %v", data["body"])
	}
}

func TestRequestIDReusesSafeHeader(t *testing.T) {
	headers := map[string]string{idgen.HeaderRequestID: "req-safe_123"}
	recorder := performRequest(testRouter(), nethttp.MethodGet, "/api/v1/system/ping", nil, headers)

	if got := recorder.Header().Get(idgen.HeaderRequestID); got != "req-safe_123" {
		t.Fatalf("unexpected response request id: %s", got)
	}

	body := decodeAPIResponse(t, recorder)
	if body["requestId"] != "req-safe_123" {
		t.Fatalf("unexpected body request id: %v", body["requestId"])
	}
}

func TestRequestIDRegeneratesUnsafeHeader(t *testing.T) {
	headers := map[string]string{idgen.HeaderRequestID: "unsafe request id ###"}
	recorder := performRequest(testRouter(), nethttp.MethodGet, "/api/v1/system/ping", nil, headers)

	got := recorder.Header().Get(idgen.HeaderRequestID)
	if got == "" {
		t.Fatal("expected response request id")
	}
	if got == "unsafe request id ###" {
		t.Fatal("expected regenerated request id")
	}
	if !idgen.ValidRequestID(got) {
		t.Fatalf("expected valid regenerated request id: %s", got)
	}
}

func TestHealthEndpoint(t *testing.T) {
	recorder := performRequest(testRouter(), nethttp.MethodGet, "/actuator/health", nil, nil)
	if recorder.Code != nethttp.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["status"] != "UP" {
		t.Fatalf("unexpected status body: %v", body["status"])
	}
}

func TestHealthHeadEndpoint(t *testing.T) {
	recorder := performRequest(testRouter(), nethttp.MethodHead, "/actuator/health", nil, nil)
	if recorder.Code != nethttp.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if recorder.Header().Get(idgen.HeaderRequestID) == "" {
		t.Fatal("expected request id response header")
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("expected empty response body, got %q", recorder.Body.String())
	}
}

func TestInfoEndpoint(t *testing.T) {
	recorder := performRequest(testRouter(), nethttp.MethodGet, "/actuator/info", nil, nil)
	if recorder.Code != nethttp.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}

	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	app := body["app"].(map[string]any)
	if app["name"] != "eventhub-backend" {
		t.Fatalf("unexpected app name: %v", app["name"])
	}
}

func TestInfoHeadEndpoint(t *testing.T) {
	recorder := performRequest(testRouter(), nethttp.MethodHead, "/actuator/info", nil, nil)
	if recorder.Code != nethttp.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if recorder.Header().Get(idgen.HeaderRequestID) == "" {
		t.Fatal("expected request id response header")
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("expected empty response body, got %q", recorder.Body.String())
	}
}

func TestOpenAPIEndpointsAreAvailableWhenEnabled(t *testing.T) {
	router := testRouterWithOpenAPI(true)

	spec := performRequest(router, nethttp.MethodGet, "/openapi.yaml", nil, nil)
	if spec.Code != nethttp.StatusOK {
		t.Fatalf("unexpected openapi status: %d body=%s", spec.Code, spec.Body.String())
	}
	if contentType := spec.Header().Get("Content-Type"); !strings.Contains(contentType, "application/yaml") {
		t.Fatalf("unexpected openapi content type: %s", contentType)
	}
	if body := spec.Body.String(); !strings.Contains(body, "openapi: 3.0.3") ||
		!strings.Contains(body, "/api/v1/auth/register") {
		t.Fatalf("unexpected openapi body: %s", body)
	}

	swagger := performRequest(router, nethttp.MethodGet, "/swagger/", nil, nil)
	if swagger.Code != nethttp.StatusOK {
		t.Fatalf("unexpected swagger status: %d body=%s", swagger.Code, swagger.Body.String())
	}
	if contentType := swagger.Header().Get("Content-Type"); !strings.Contains(contentType, "text/html") {
		t.Fatalf("unexpected swagger content type: %s", contentType)
	}
	if body := swagger.Body.String(); !strings.Contains(body, "SwaggerUIBundle") ||
		!strings.Contains(body, "/openapi.yaml") ||
		!strings.Contains(body, "/swagger/swagger-ui.css") ||
		!strings.Contains(body, "/swagger/swagger-ui-bundle.js") {
		t.Fatalf("unexpected swagger body: %s", body)
	}
	assertNoExternalSwaggerCDN(t, swagger.Body.String())
}

func TestOpenAPIEndpointsServeLocalStaticAssetsWhenEnabled(t *testing.T) {
	root := writeOpenAPITestAssets(t)
	router := testRouterWithOpenAPIAssets(root)

	spec := performRequest(router, nethttp.MethodGet, "/openapi.yaml", nil, nil)
	if spec.Code != nethttp.StatusOK {
		t.Fatalf("unexpected openapi status: %d body=%s", spec.Code, spec.Body.String())
	}
	if got := spec.Body.String(); got != testOpenAPIYAML {
		t.Fatalf("expected local openapi.yaml body, got %q", got)
	}

	swagger := performRequest(router, nethttp.MethodGet, "/swagger/", nil, nil)
	if swagger.Code != nethttp.StatusOK {
		t.Fatalf("unexpected swagger status: %d body=%s", swagger.Code, swagger.Body.String())
	}
	if got := swagger.Body.String(); got != testSwaggerHTML {
		t.Fatalf("expected local swagger html body, got %q", got)
	}
	assertNoExternalSwaggerCDN(t, swagger.Body.String())

	css := performRequest(router, nethttp.MethodGet, "/swagger/swagger-ui.css", nil, nil)
	if css.Code != nethttp.StatusOK {
		t.Fatalf("unexpected swagger css status: %d body=%s", css.Code, css.Body.String())
	}
	if contentType := css.Header().Get("Content-Type"); !strings.Contains(contentType, "text/css") {
		t.Fatalf("unexpected swagger css content type: %s", contentType)
	}
	if got := css.Body.String(); got != testSwaggerCSS {
		t.Fatalf("expected local swagger css body, got %q", got)
	}

	js := performRequest(router, nethttp.MethodGet, "/swagger/swagger-ui-bundle.js", nil, nil)
	if js.Code != nethttp.StatusOK {
		t.Fatalf("unexpected swagger js status: %d body=%s", js.Code, js.Body.String())
	}
	if contentType := js.Header().Get("Content-Type"); !strings.Contains(contentType, "javascript") {
		t.Fatalf("unexpected swagger js content type: %s", contentType)
	}
	if got := js.Body.String(); got != testSwaggerJS {
		t.Fatalf("expected local swagger js body, got %q", got)
	}
}

func TestOpenAPIEndpointsAreNotRegisteredWhenDisabled(t *testing.T) {
	router := testRouterWithOpenAPI(false)

	spec := performRequest(router, nethttp.MethodGet, "/openapi.yaml", nil, nil)
	if spec.Code != nethttp.StatusNotFound {
		t.Fatalf("unexpected openapi status: %d body=%s", spec.Code, spec.Body.String())
	}
	if body := decodeAPIResponse(t, spec); body["code"] != "COMMON-404" {
		t.Fatalf("unexpected openapi body: %#v", body)
	}

	for _, path := range []string{
		"/swagger/",
		"/swagger/index.html",
		"/swagger/swagger-ui.css",
		"/swagger/swagger-ui-bundle.js",
	} {
		swagger := performRequest(router, nethttp.MethodGet, path, nil, nil)
		if swagger.Code != nethttp.StatusNotFound {
			t.Fatalf("unexpected swagger status for %s: %d body=%s", path, swagger.Code, swagger.Body.String())
		}
		if body := decodeAPIResponse(t, swagger); body["code"] != "COMMON-404" {
			t.Fatalf("unexpected swagger body for %s: %#v", path, body)
		}
	}
}

func TestOpenAPIDeclaredRoutesUseGeneratedStrictRouter(t *testing.T) {
	recorder := performRequest(
		testRouter(),
		nethttp.MethodPost,
		"/api/v1/auth/login",
		[]byte(`{"usernameOrEmail":"alice"`),
		map[string]string{"Content-Type": "application/json"},
	)

	if recorder.Code != nethttp.StatusBadRequest {
		t.Fatalf("unexpected status: %d body=%s", recorder.Code, recorder.Body.String())
	}
	body := decodeAPIResponse(t, recorder)
	if body["code"] != "COMMON-400" {
		t.Fatalf("unexpected code: %v", body["code"])
	}
	if body["message"] != "请求体格式不合法" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
}

func TestMissingRouteReturnsUnifiedNotFound(t *testing.T) {
	recorder := performRequest(testRouter(), nethttp.MethodGet, "/favicon.ico", nil, nil)
	if recorder.Code != nethttp.StatusNotFound {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}

	body := decodeAPIResponse(t, recorder)
	if body["code"] != "COMMON-404" {
		t.Fatalf("unexpected code: %v", body["code"])
	}
	if body["message"] != "请求的资源不存在" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
}

func TestPanicRecoverReturnsUnifiedInternalError(t *testing.T) {
	router := chi.NewRouter()
	logger := testLogger()
	router.Use(middleware.RequestID(logger))
	router.Use(middleware.Recover(logger))
	router.Get("/panic", func(w nethttp.ResponseWriter, r *nethttp.Request) {
		panic("boom")
	})

	recorder := performRequest(router, nethttp.MethodGet, "/panic", nil, nil)
	if recorder.Code != nethttp.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}

	body := decodeAPIResponse(t, recorder)
	if body["code"] != "COMMON-500" {
		t.Fatalf("unexpected code: %v", body["code"])
	}
	if body["message"] != "系统内部错误" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
	if body["requestId"] == "" {
		t.Fatal("expected request id in panic response")
	}
}

func TestPanicRecoverDoesNotWriteErrorAfterCommittedResponse(t *testing.T) {
	tests := []struct {
		name       string
		handler    nethttp.HandlerFunc
		wantStatus int
		wantBody   string
	}{
		{
			name: "explicit header committed",
			handler: func(w nethttp.ResponseWriter, r *nethttp.Request) {
				w.WriteHeader(nethttp.StatusAccepted)
				panic("boom after header")
			},
			wantStatus: nethttp.StatusAccepted,
			wantBody:   "",
		},
		{
			name: "body committed",
			handler: func(w nethttp.ResponseWriter, r *nethttp.Request) {
				_, _ = w.Write([]byte("partial response"))
				panic("boom after body")
			},
			wantStatus: nethttp.StatusOK,
			wantBody:   "partial response",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router := chi.NewRouter()
			logger := testLogger()
			router.Use(middleware.RequestID(logger))
			router.Use(middleware.Recover(logger))
			router.Get("/panic-after-commit", tt.handler)

			recorder := performRequest(router, nethttp.MethodGet, "/panic-after-commit", nil, nil)
			if recorder.Code != tt.wantStatus {
				t.Fatalf("unexpected status: got %d want %d", recorder.Code, tt.wantStatus)
			}
			if got := recorder.Body.String(); got != tt.wantBody {
				t.Fatalf("unexpected body: got %q want %q", got, tt.wantBody)
			}
			if bytes.Contains(recorder.Body.Bytes(), []byte("COMMON-500")) {
				t.Fatal("did not expect COMMON-500 to be appended after response was committed")
			}
			if recorder.Header().Get(idgen.HeaderRequestID) == "" {
				t.Fatal("expected request id response header")
			}
		})
	}
}

func testRouter() nethttp.Handler {
	return testRouterWithOpenAPI(false)
}

func testRouterWithOpenAPI(enabled bool) nethttp.Handler {
	var openAPI *openapihandler.OpenAPIHandler
	if enabled {
		var err error
		openAPI, err = openapihandler.NewOpenAPIHandler(repoOpenAPIAssetRoot())
		if err != nil {
			panic(err)
		}
	}
	return testRouterWithOpenAPIHandler(openAPI)
}

func testRouterWithOpenAPIAssets(assetRoot string) nethttp.Handler {
	openAPI, err := openapihandler.NewOpenAPIHandler(assetRoot)
	if err != nil {
		panic(err)
	}
	return testRouterWithOpenAPIHandler(openAPI)
}

func repoOpenAPIAssetRoot() string {
	return filepath.Join("..", "..", filepath.FromSlash(openapispec.AssetRoot))
}

func testRouterWithOpenAPIHandler(openAPI *openapihandler.OpenAPIHandler) nethttp.Handler {
	cfg := config.Config{
		AppName: "eventhub-backend",
		Env:     config.EnvTest,
		Version: "test",
		Log:     config.LogConfig{Level: slog.LevelError},
	}
	systemService := systemsvc.NewService(cfg, clock.RealClock{})
	return apphttp.NewRouter(testLogger(), apphttp.RouterDependencies{
		System:  systemhandler.NewHandler(systemService),
		OpenAPI: openAPI,
	})
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func performRequest(handler nethttp.Handler, method, path string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	request := httptest.NewRequest(method, path, reader)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)
	return recorder
}

func decodeAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["requestId"] == "" {
		t.Fatal("expected requestId")
	}
	if body["timestamp"] == "" {
		t.Fatal("expected timestamp")
	}
	return body
}

const (
	testOpenAPIYAML = "openapi: 3.0.3\ninfo:\n  title: Local Test API\n  version: test\n"
	testSwaggerHTML = "<!doctype html><title>Local Test Swagger UI</title><div id=\"swagger-ui\"></div>\n"
	testSwaggerCSS  = ".swagger-ui { color: #111; }\n"
	testSwaggerJS   = "window.SwaggerUIBundle = function () {};\n"
)

func writeOpenAPITestAssets(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	swaggerDir := filepath.Join(root, filepath.FromSlash(openapispec.SwaggerDirPath))
	if err := os.MkdirAll(swaggerDir, 0o755); err != nil {
		t.Fatalf("create swagger asset dir: %v", err)
	}
	writeTestFile(t, filepath.Join(root, filepath.FromSlash(openapispec.SpecPath)), testOpenAPIYAML)
	writeTestFile(t, filepath.Join(root, filepath.FromSlash(openapispec.SwaggerIndexPath)), testSwaggerHTML)
	writeTestFile(t, filepath.Join(root, filepath.FromSlash(openapispec.SwaggerCSSPath)), testSwaggerCSS)
	writeTestFile(t, filepath.Join(root, filepath.FromSlash(openapispec.SwaggerBundlePath)), testSwaggerJS)
	return root
}

func writeTestFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func assertNoExternalSwaggerCDN(t *testing.T, body string) {
	t.Helper()
	for _, forbidden := range []string{"https://cdn", "unpkg", "jsdelivr"} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("swagger ui html must not reference external CDN %q: %s", forbidden, body)
		}
	}
}
