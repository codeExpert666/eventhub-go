package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"eventhub-go/internal/security"
)

func TestRequireRoleAllowsMatchingRole(t *testing.T) {
	handler := RequireRole("ADMIN")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	request = request.WithContext(security.ContextWithPrincipal(request.Context(), security.Principal{
		UserID:      1001,
		Username:    "admin",
		Authorities: []string{"ROLE_ADMIN", "ROLE_USER"},
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
}

func TestRequireRoleRejectsMissingRole(t *testing.T) {
	handler := RequireRole("ADMIN")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil)
	request = request.WithContext(security.ContextWithPrincipal(request.Context(), security.Principal{
		UserID:      1002,
		Username:    "alice",
		Authorities: []string{"ROLE_USER"},
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, request)

	assertRBACError(t, recorder, http.StatusForbidden, "AUTH-403")
}

func TestRequireRoleRejectsMissingPrincipal(t *testing.T) {
	handler := RequireRole("ADMIN")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler should not be called")
	}))
	recorder := httptest.NewRecorder()

	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/users", nil))

	assertRBACError(t, recorder, http.StatusUnauthorized, "AUTH-401")
}

func assertRBACError(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("unexpected status: %d body=%s", recorder.Code, recorder.Body.String())
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

func TestNormalizeAuthorityAcceptsPrefixedRole(t *testing.T) {
	if got := normalizeAuthority("role_admin"); got != "ROLE_ADMIN" {
		t.Fatalf("unexpected authority: %s", got)
	}
}

func TestHasAuthorityTrimsAndNormalizesCase(t *testing.T) {
	if !hasAuthority([]string{" role_admin "}, "ROLE_ADMIN") {
		t.Fatal("expected authority match")
	}
}

func TestRequireRoleWorksWithContextCreatedElsewhere(t *testing.T) {
	ctx := security.ContextWithPrincipal(context.Background(), security.Principal{
		UserID:      1003,
		Username:    "admin",
		Authorities: []string{"ROLE_ADMIN"},
	})
	if _, ok := security.PrincipalFromContext(ctx); !ok {
		t.Fatal("expected principal from context")
	}
}
