package http_test

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"

	"eventhub-go/internal/config"
	apphttp "eventhub-go/internal/http"
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
	cfg := config.Config{
		AppName: "eventhub-backend",
		Env:     config.EnvTest,
		Version: "test",
		Log:     config.LogConfig{Level: slog.LevelError},
	}
	systemService := systemsvc.NewService(cfg, clock.RealClock{})
	return apphttp.NewRouter(testLogger(), apphttp.RouterDependencies{
		System: systemhandler.NewHandler(systemService),
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
