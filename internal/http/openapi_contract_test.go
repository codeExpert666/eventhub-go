package http_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	legacyrouter "github.com/getkin/kin-openapi/routers/legacy"

	openapispec "eventhub-go/api/openapi"
)

func TestOpenAPIResponseContractsValidateRealRouterResponses(t *testing.T) {
	router, _ := testAuthRouter(t)
	specRouter := loadOpenAPIResponseContractRouter(t)

	tests := []struct {
		name       string
		method     string
		path       string
		body       []byte
		headers    map[string]string
		wantStatus int
	}{
		{
			name:       "actuator health",
			method:     http.MethodGet,
			path:       "/actuator/health",
			wantStatus: http.StatusOK,
		},
		{
			name:       "actuator info",
			method:     http.MethodGet,
			path:       "/actuator/info",
			wantStatus: http.StatusOK,
		},
		{
			name:       "system ping",
			method:     http.MethodGet,
			path:       "/api/v1/system/ping",
			wantStatus: http.StatusOK,
		},
		{
			name:       "system echo",
			method:     http.MethodPost,
			path:       "/api/v1/system/echo",
			body:       jsonBody(t, map[string]string{"message": "hello eventhub", "tag": "contract"}),
			headers:    jsonHeaders(),
			wantStatus: http.StatusOK,
		},
		{
			name:       "me without token",
			method:     http.MethodGet,
			path:       "/api/v1/me",
			wantStatus: http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := newOpenAPIContractRequest(t, tt.method, tt.path, tt.body, tt.headers)
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, request)

			if recorder.Code != tt.wantStatus {
				t.Fatalf("%s %s status mismatch: got %d want %d body=%s", tt.method, tt.path, recorder.Code, tt.wantStatus, compactContractBody(recorder.Body.String()))
			}

			validateOpenAPIResponseContract(t, specRouter, request, recorder)
		})
	}
}

func loadOpenAPIResponseContractRouter(t *testing.T) routers.Router {
	t.Helper()

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(openapispec.SpecYAML())
	if err != nil {
		t.Fatalf("load api/openapi/eventhub.yaml: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("validate api/openapi/eventhub.yaml: %v", err)
	}

	router, err := legacyrouter.NewRouter(doc)
	if err != nil {
		t.Fatalf("create OpenAPI response contract router: %v", err)
	}
	return router
}

func newOpenAPIContractRequest(t *testing.T, method, path string, body []byte, headers map[string]string) *http.Request {
	t.Helper()

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader(body)
	}

	request := httptest.NewRequest(method, "http://localhost:8080"+path, reader)
	for key, value := range headers {
		request.Header.Set(key, value)
	}
	return request
}

func validateOpenAPIResponseContract(t *testing.T, specRouter routers.Router, request *http.Request, recorder *httptest.ResponseRecorder) {
	t.Helper()

	route, pathParams, err := specRouter.FindRoute(request)
	if err != nil {
		t.Fatalf("match OpenAPI route %s %s: %v", request.Method, request.URL.Path, err)
	}

	result := recorder.Result()
	defer result.Body.Close()

	input := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    request,
			PathParams: pathParams,
			Route:      route,
		},
		Status: recorder.Code,
		Header: result.Header.Clone(),
		Options: &openapi3filter.Options{
			IncludeResponseStatus: true,
		},
	}
	input.SetBodyBytes(recorder.Body.Bytes())

	if err := openapi3filter.ValidateResponse(context.Background(), input); err != nil {
		t.Fatalf(
			"validate OpenAPI response %s %s status=%d content-type=%q body=%s: %v",
			request.Method,
			request.URL.Path,
			recorder.Code,
			result.Header.Get("Content-Type"),
			compactContractBody(recorder.Body.String()),
			err,
		)
	}
}

func compactContractBody(body string) string {
	body = strings.TrimSpace(body)
	if body == "" {
		return "<empty>"
	}

	body = strings.Join(strings.Fields(body), " ")
	const maxBodyLength = 320
	if len(body) <= maxBodyLength {
		return body
	}
	return body[:maxBodyLength] + "...(truncated)"
}
