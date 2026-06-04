package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/security"
	"eventhub-go/internal/security/jwt"
)

const testSigningSecret = "eventhub-test-access-token-secret-for-auth-tests"

func TestAuthMiddlewareRejectsMissingToken(t *testing.T) {
	codec := newTestJWTCodec(t)
	loader := &testPrincipalLoader{}
	handler := NewAuth(codec, loader).Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/me", nil))

	assertAPIError(t, recorder, http.StatusUnauthorized, "AUTH-401")
}

func TestAuthMiddlewareStoresPrincipalInContext(t *testing.T) {
	codec := newTestJWTCodec(t)
	token, err := codec.IssueAccessToken(1001, "session-1001", 30*time.Minute)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	loader := &testPrincipalLoader{
		principal: security.Principal{
			UserID:      1001,
			Username:    "alice",
			Authorities: []string{"ROLE_USER"},
		},
	}
	handler := NewAuth(codec, loader).Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		principal, ok := security.PrincipalFromContext(r.Context())
		if !ok {
			t.Fatal("expected principal in context")
		}
		if principal.UserID != 1001 || principal.Username != "alice" {
			t.Fatalf("unexpected principal: %#v", principal)
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	if loader.loadedUserID != 1001 {
		t.Fatalf("expected loader to use sub user id, got %d", loader.loadedUserID)
	}
}

func TestAuthMiddlewareRejectsDisabledUserOldToken(t *testing.T) {
	codec := newTestJWTCodec(t)
	token, err := codec.IssueAccessToken(1001, "session-1001", 30*time.Minute)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	loader := &testPrincipalLoader{
		err: apperror.New(apperror.AuthUnauthorized, "请先登录或重新登录"),
	}
	handler := NewAuth(codec, loader).Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	assertAPIError(t, recorder, http.StatusUnauthorized, "AUTH-401")
}

func TestAuthMiddlewareRejectsExpiredToken(t *testing.T) {
	codec := newTestJWTCodec(t)
	token, err := codec.IssueAccessToken(1001, "session-1001", -time.Second)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	handler := NewAuth(codec, &testPrincipalLoader{}).Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))

	request := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	assertAPIError(t, recorder, http.StatusUnauthorized, "AUTH-401")
}

func newTestJWTCodec(t *testing.T) *jwt.Codec {
	t.Helper()
	codec, err := jwt.NewCodec("eventhub-backend", testSigningSecret, nil)
	if err != nil {
		t.Fatalf("new jwt codec: %v", err)
	}
	return codec
}

type testPrincipalLoader struct {
	principal    security.Principal
	loadedUserID int64
	err          error
}

func (l *testPrincipalLoader) LoadPrincipal(ctx context.Context, userID int64) (security.Principal, error) {
	l.loadedUserID = userID
	if l.err != nil {
		return security.Principal{}, l.err
	}
	return l.principal, nil
}

func assertAPIError(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Code != code {
		t.Fatalf("unexpected code: %s", body.Code)
	}
}
