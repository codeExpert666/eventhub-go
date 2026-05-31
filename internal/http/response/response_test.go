package response_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/response"
	"eventhub-go/internal/platform/idgen"
)

func TestWriteSuccess(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	request = request.WithContext(idgen.WithRequestID(request.Context(), "req-success"))
	recorder := httptest.NewRecorder()

	response.WriteSuccess(recorder, request, map[string]string{"hello": "eventhub"})

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	body := decodeBody(t, recorder)
	if body["code"] != "COMMON-000" {
		t.Fatalf("unexpected code: %v", body["code"])
	}
	if body["message"] != "成功" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
	if body["requestId"] != "req-success" {
		t.Fatalf("unexpected requestId: %v", body["requestId"])
	}
	if body["timestamp"] == "" {
		t.Fatal("expected timestamp")
	}
}

func TestWriteError(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	request = request.WithContext(idgen.WithRequestID(request.Context(), "req-error"))
	recorder := httptest.NewRecorder()

	response.WriteError(recorder, request, apperror.New(apperror.AuthForbidden, "权限不足"))

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	body := decodeBody(t, recorder)
	if body["code"] != "AUTH-403" {
		t.Fatalf("unexpected code: %v", body["code"])
	}
	if body["message"] != "权限不足" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
	if body["requestId"] != "req-error" {
		t.Fatalf("unexpected requestId: %v", body["requestId"])
	}
}

func decodeBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}
