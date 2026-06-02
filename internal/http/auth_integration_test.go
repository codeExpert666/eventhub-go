package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/config"
	apphttp "eventhub-go/internal/http"
	"eventhub-go/internal/http/middleware"
	"eventhub-go/internal/repository"
	"eventhub-go/internal/security"
	"eventhub-go/internal/security/jwt"
	authsvc "eventhub-go/internal/service/auth"
	usersvc "eventhub-go/internal/service/user"
)

func TestAuthRegisterEndpoint(t *testing.T) {
	router, _ := testAuthRouter(t)

	recorder := performRequest(router, http.MethodPost, "/api/v1/auth/register", jsonBody(t, map[string]string{
		"username": "alice",
		"email":    "alice@example.com",
		"password": "Password123",
	}), jsonHeaders())

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	body := decodeAPIResponse(t, recorder)
	if body["code"] != "COMMON-000" {
		t.Fatalf("unexpected code: %v", body["code"])
	}
	data := body["data"].(map[string]any)
	if data["username"] != "alice" || data["email"] != "alice@example.com" || data["status"] != "ENABLED" {
		t.Fatalf("unexpected data: %#v", data)
	}
	roles := data["roles"].([]any)
	if len(roles) != 1 || roles[0] != "USER" {
		t.Fatalf("unexpected roles: %#v", roles)
	}
	if _, ok := data["passwordHash"]; ok {
		t.Fatal("passwordHash must not be exposed")
	}
}

func TestAuthRegisterEndpointRejectsDuplicateUsername(t *testing.T) {
	router, _ := testAuthRouter(t)
	registerViaHTTP(t, router, "alice", "alice@example.com")

	recorder := performRequest(router, http.MethodPost, "/api/v1/auth/register", jsonBody(t, map[string]string{
		"username": "alice",
		"email":    "alice2@example.com",
		"password": "Password123",
	}), jsonHeaders())

	assertHTTPError(t, recorder, http.StatusConflict, "AUTH-409", "用户名已存在")
}

func TestAuthLoginEndpoint(t *testing.T) {
	router, _ := testAuthRouter(t)
	registerViaHTTP(t, router, "alice", "alice@example.com")

	recorder := performRequest(router, http.MethodPost, "/api/v1/auth/login", jsonBody(t, map[string]string{
		"usernameOrEmail": "alice",
		"password":        "Password123",
	}), jsonHeaders())

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	data := decodeAPIResponse(t, recorder)["data"].(map[string]any)
	if data["accessToken"] == "" || data["refreshToken"] == "" || data["sessionId"] == "" {
		t.Fatalf("expected tokens and session id: %#v", data)
	}
	if data["authorizationScheme"] != "Bearer" {
		t.Fatalf("unexpected scheme: %v", data["authorizationScheme"])
	}
	if data["expiresIn"].(float64) <= 0 || data["refreshExpiresIn"].(float64) != 2592000 {
		t.Fatalf("unexpected ttls: %#v", data)
	}
	user := data["user"].(map[string]any)
	if user["username"] != "alice" {
		t.Fatalf("unexpected user: %#v", user)
	}
}

func TestAuthLoginEndpointRejectsWrongPassword(t *testing.T) {
	router, _ := testAuthRouter(t)
	registerViaHTTP(t, router, "alice", "alice@example.com")

	recorder := performRequest(router, http.MethodPost, "/api/v1/auth/login", jsonBody(t, map[string]string{
		"usernameOrEmail": "alice",
		"password":        "WrongPassword123",
	}), jsonHeaders())

	assertHTTPError(t, recorder, http.StatusUnauthorized, "AUTH-401", "账号或密码错误")
}

func TestMeEndpointReturnsCurrentUser(t *testing.T) {
	router, _ := testAuthRouter(t)
	registerViaHTTP(t, router, "alice", "alice@example.com")
	token := loginAndReturnAccessToken(t, router, "alice", "Password123")

	recorder := performRequest(router, http.MethodGet, "/api/v1/me", nil, map[string]string{
		"Authorization": "Bearer " + token,
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	data := decodeAPIResponse(t, recorder)["data"].(map[string]any)
	if data["username"] != "alice" {
		t.Fatalf("unexpected current user: %#v", data)
	}
	roles := data["roles"].([]any)
	if len(roles) != 1 || roles[0] != "USER" {
		t.Fatalf("unexpected roles: %#v", roles)
	}
}

func TestMeEndpointRejectsDisabledUserOldToken(t *testing.T) {
	router, auth := testAuthRouter(t)
	registerViaHTTP(t, router, "alice", "alice@example.com")
	token := loginAndReturnAccessToken(t, router, "alice", "Password123")
	auth.disableUser("alice")

	recorder := performRequest(router, http.MethodGet, "/api/v1/me", nil, map[string]string{
		"Authorization": "Bearer " + token,
	})

	assertHTTPError(t, recorder, http.StatusUnauthorized, "AUTH-401", "请先登录或重新登录")
}

func testAuthRouter(t *testing.T) (http.Handler, *testHTTPAuthService) {
	t.Helper()
	codec, err := jwt.NewCodec(jwt.Config{
		Issuer:        "eventhub-backend",
		SigningSecret: "eventhub-test-access-token-secret-for-auth-tests",
		AccessTTL:     30 * time.Minute,
	})
	if err != nil {
		t.Fatalf("new jwt codec: %v", err)
	}
	auth := newTestHTTPAuthService(codec)
	router := apphttp.NewRouter(
		config.Config{AppName: "eventhub-backend", Version: "test"},
		testLogger(),
		apphttp.WithAuth(auth, auth, middleware.NewAuth(codec, auth)),
	)
	return router, auth
}

func registerViaHTTP(t *testing.T, router http.Handler, username, email string) {
	t.Helper()
	recorder := performRequest(router, http.MethodPost, "/api/v1/auth/register", jsonBody(t, map[string]string{
		"username": username,
		"email":    email,
		"password": "Password123",
	}), jsonHeaders())
	if recorder.Code != http.StatusOK {
		t.Fatalf("register status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func loginAndReturnAccessToken(t *testing.T, router http.Handler, usernameOrEmail, password string) string {
	t.Helper()
	recorder := performRequest(router, http.MethodPost, "/api/v1/auth/login", jsonBody(t, map[string]string{
		"usernameOrEmail": usernameOrEmail,
		"password":        password,
	}), jsonHeaders())
	if recorder.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	data := decodeAPIResponse(t, recorder)["data"].(map[string]any)
	token, ok := data["accessToken"].(string)
	if !ok || token == "" {
		t.Fatalf("missing accessToken: %#v", data)
	}
	return token
}

func jsonBody(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return body
}

func jsonHeaders() map[string]string {
	return map[string]string{"Content-Type": "application/json"}
}

func assertHTTPError(t *testing.T, recorder *httptest.ResponseRecorder, status int, code, message string) {
	t.Helper()
	if recorder.Code != status {
		t.Fatalf("unexpected status: %d body=%s", recorder.Code, recorder.Body.String())
	}
	body := decodeAPIResponse(t, recorder)
	if body["code"] != code || body["message"] != message {
		t.Fatalf("unexpected error body: %#v", body)
	}
}

type testHTTPAuthService struct {
	mu       sync.Mutex
	nextID   int64
	codec    *jwt.Codec
	users    map[int64]*testHTTPUser
	username map[string]int64
	email    map[string]int64
}

type testHTTPUser struct {
	id           int64
	username     string
	email        string
	passwordHash string
	status       repository.UserStatus
	roles        []string
}

func newTestHTTPAuthService(codec *jwt.Codec) *testHTTPAuthService {
	return &testHTTPAuthService{
		nextID:   1,
		codec:    codec,
		users:    map[int64]*testHTTPUser{},
		username: map[string]int64{},
		email:    map[string]int64{},
	}
}

func (s *testHTTPAuthService) Register(ctx context.Context, command authsvc.RegisterCommand) (usersvc.UserResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	username := strings.TrimSpace(command.Username)
	email := strings.ToLower(strings.TrimSpace(command.Email))
	if _, ok := s.username[username]; ok {
		return usersvc.UserResult{}, apperror.New(apperror.AuthConflict, "用户名已存在")
	}
	if _, ok := s.email[email]; ok {
		return usersvc.UserResult{}, apperror.New(apperror.AuthConflict, "邮箱已存在")
	}
	user := &testHTTPUser{
		id:           s.nextID,
		username:     username,
		email:        email,
		passwordHash: "hash:" + command.Password,
		status:       repository.UserStatusEnabled,
		roles:        []string{"USER"},
	}
	s.nextID++
	s.users[user.id] = user
	s.username[user.username] = user.id
	s.email[user.email] = user.id
	return user.toResult(), nil
}

func (s *testHTTPAuthService) Login(ctx context.Context, command authsvc.LoginCommand) (authsvc.LoginResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.findUserLocked(command.UsernameOrEmail)
	if user == nil || user.passwordHash != "hash:"+command.Password {
		return authsvc.LoginResult{}, apperror.New(apperror.AuthUnauthorized, "账号或密码错误")
	}
	if user.status == repository.UserStatusDisabled {
		return authsvc.LoginResult{}, apperror.New(apperror.AuthForbidden, "用户已被禁用")
	}
	sessionID := uuid.NewString()
	accessToken, err := s.codec.IssueAccessToken(user.id, sessionID)
	if err != nil {
		return authsvc.LoginResult{}, err
	}
	return authsvc.LoginResult{
		AccessToken:         accessToken,
		RefreshToken:        strings.Repeat("a", 43),
		AuthorizationScheme: "Bearer",
		ExpiresIn:           int64(s.codec.AccessTokenTTL().Seconds()),
		RefreshExpiresIn:    2592000,
		SessionID:           sessionID,
		User:                user.toResult(),
	}, nil
}

func (s *testHTTPAuthService) CurrentUser(ctx context.Context, query usersvc.CurrentUserQuery) (usersvc.UserResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.users[query.Principal.UserID]
	if user == nil || user.status != repository.UserStatusEnabled {
		return usersvc.UserResult{}, apperror.New(apperror.AuthUnauthorized, "请先登录或重新登录")
	}
	return user.toResult(), nil
}

func (s *testHTTPAuthService) LoadPrincipal(ctx context.Context, userID int64) (security.Principal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user := s.users[userID]
	if user == nil || user.status != repository.UserStatusEnabled {
		return security.Principal{}, apperror.New(apperror.AuthUnauthorized, "请先登录或重新登录")
	}
	return security.Principal{
		UserID:      user.id,
		Username:    user.username,
		Authorities: []string{"ROLE_USER"},
	}, nil
}

func (s *testHTTPAuthService) disableUser(username string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.username[username]; ok {
		s.users[id].status = repository.UserStatusDisabled
	}
}

func (s *testHTTPAuthService) findUserLocked(usernameOrEmail string) *testHTTPUser {
	identifier := strings.TrimSpace(usernameOrEmail)
	if strings.Contains(identifier, "@") {
		identifier = strings.ToLower(identifier)
	}
	if id, ok := s.username[identifier]; ok {
		return s.users[id]
	}
	if id, ok := s.email[identifier]; ok {
		return s.users[id]
	}
	return nil
}

func (u *testHTTPUser) toResult() usersvc.UserResult {
	return usersvc.UserResult{
		ID:       u.id,
		Username: u.username,
		Email:    u.email,
		Status:   string(u.status),
		Roles:    append([]string(nil), u.roles...),
	}
}
