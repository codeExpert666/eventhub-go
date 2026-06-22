package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"eventhub-go/internal/repository"
	"eventhub-go/internal/security"
	"eventhub-go/internal/security/jwt"
	usersvc "eventhub-go/internal/service/user"
)

const testSigningSecret = "eventhub-test-access-token-secret-for-auth-tests"

func TestAuthMiddlewareRejectsMissingToken(t *testing.T) {
	codec := newTestJWTCodec(t)
	users := newTestUserService(repository.User{ID: 1001, Username: "alice", Status: repository.UserStatusEnabled}, []string{"USER"})
	handler := Authenticate(codec, users)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	userRepo := &testUserRepository{
		user:  repository.User{ID: 1001, Username: "alice", Status: repository.UserStatusEnabled},
		found: true,
	}
	roleRepo := &testRoleRepository{roles: []string{"USER"}}
	users := usersvc.NewService(userRepo, roleRepo, nil)
	handler := Authenticate(codec, users)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	if userRepo.loadedUserID != 1001 {
		t.Fatalf("expected user service to use sub user id, got %d", userRepo.loadedUserID)
	}
}

func TestAuthMiddlewareRejectsDisabledUserOldToken(t *testing.T) {
	codec := newTestJWTCodec(t)
	token, err := codec.IssueAccessToken(1001, "session-1001", 30*time.Minute)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	users := newTestUserService(repository.User{ID: 1001, Username: "alice", Status: repository.UserStatusDisabled}, []string{"USER"})
	handler := Authenticate(codec, users)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	users := newTestUserService(repository.User{ID: 1001, Username: "alice", Status: repository.UserStatusEnabled}, []string{"USER"})
	handler := Authenticate(codec, users)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func newTestUserService(user repository.User, roles []string) *usersvc.Service {
	return usersvc.NewService(
		&testUserRepository{user: user, found: true},
		&testRoleRepository{roles: roles},
		nil,
	)
}

type testUserRepository struct {
	user         repository.User
	found        bool
	loadedUserID int64
}

func (r *testUserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	return false, nil
}

func (r *testUserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	return false, nil
}

func (r *testUserRepository) Create(ctx context.Context, input repository.CreateUserInput) (repository.User, error) {
	return repository.User{}, nil
}

func (r *testUserRepository) FindByUsernameOrEmail(ctx context.Context, usernameOrEmail string) (repository.User, bool, error) {
	return repository.User{}, false, nil
}

func (r *testUserRepository) FindByID(ctx context.Context, id int64) (repository.User, bool, error) {
	r.loadedUserID = id
	return r.user, r.found, nil
}

func (r *testUserRepository) CountByCriteria(ctx context.Context, criteria repository.UserCriteria) (int64, error) {
	return 0, nil
}

func (r *testUserRepository) ListUsers(ctx context.Context, criteria repository.UserCriteria, limit int32, offset int32) ([]repository.User, error) {
	return nil, nil
}

func (r *testUserRepository) UpdateStatus(ctx context.Context, id int64, status repository.UserStatus) (int64, error) {
	return 0, nil
}

type testRoleRepository struct {
	roles []string
}

func (r *testRoleRepository) FindByCode(ctx context.Context, code string) (repository.Role, bool, error) {
	return repository.Role{}, false, nil
}

func (r *testRoleRepository) FindRoleCodesByUserID(ctx context.Context, userID int64) ([]string, error) {
	return r.roles, nil
}

func (r *testRoleRepository) FindRoleCodesByUserIDs(ctx context.Context, userIDs []int64) ([]repository.UserRoleCode, error) {
	return nil, nil
}

func (r *testRoleRepository) AddRoleToUser(ctx context.Context, userID, roleID int64) (int64, error) {
	return 0, nil
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
