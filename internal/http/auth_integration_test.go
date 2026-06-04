package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"eventhub-go/internal/config"
	apphttp "eventhub-go/internal/http"
	authhandler "eventhub-go/internal/http/handler/auth"
	systemhandler "eventhub-go/internal/http/handler/system"
	userhandler "eventhub-go/internal/http/handler/user"
	"eventhub-go/internal/http/middleware"
	"eventhub-go/internal/platform/clock"
	"eventhub-go/internal/repository"
	"eventhub-go/internal/security/jwt"
	"eventhub-go/internal/security/password"
	"eventhub-go/internal/security/refresh"
	authsvc "eventhub-go/internal/service/auth"
	systemsvc "eventhub-go/internal/service/system"
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

func TestAuthRefreshEndpointRotatesTokenPair(t *testing.T) {
	router, _ := testAuthRouter(t)
	registerViaHTTP(t, router, "alice", "alice@example.com")
	login := loginAndReturnTokenPair(t, router, "alice", "Password123")

	recorder := performRequest(router, http.MethodPost, "/api/v1/auth/refresh", jsonBody(t, map[string]string{
		"refreshToken": login.refreshToken,
	}), jsonHeaders())

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", recorder.Code, recorder.Body.String())
	}
	data := decodeAPIResponse(t, recorder)["data"].(map[string]any)
	if data["accessToken"] == "" || data["refreshToken"] == "" || data["sessionId"] != login.sessionID {
		t.Fatalf("expected refreshed token pair: %#v", data)
	}
	if data["refreshToken"] == login.refreshToken {
		t.Fatal("expected refresh token rotation")
	}
	if data["authorizationScheme"] != "Bearer" {
		t.Fatalf("unexpected scheme: %v", data["authorizationScheme"])
	}
	user := data["user"].(map[string]any)
	if user["username"] != "alice" {
		t.Fatalf("unexpected user: %#v", user)
	}
}

func TestAuthRefreshEndpointRejectsReplay(t *testing.T) {
	router, _ := testAuthRouter(t)
	registerViaHTTP(t, router, "alice", "alice@example.com")
	login := loginAndReturnTokenPair(t, router, "alice", "Password123")

	first := performRequest(router, http.MethodPost, "/api/v1/auth/refresh", jsonBody(t, map[string]string{
		"refreshToken": login.refreshToken,
	}), jsonHeaders())
	if first.Code != http.StatusOK {
		t.Fatalf("first refresh status=%d body=%s", first.Code, first.Body.String())
	}

	replay := performRequest(router, http.MethodPost, "/api/v1/auth/refresh", jsonBody(t, map[string]string{
		"refreshToken": login.refreshToken,
	}), jsonHeaders())

	assertHTTPError(t, replay, http.StatusUnauthorized, "AUTH-401", "refresh token 无效或已过期")
}

func TestAuthRefreshEndpointRejectsBlankRefreshToken(t *testing.T) {
	router, _ := testAuthRouter(t)

	recorder := performRequest(router, http.MethodPost, "/api/v1/auth/refresh", jsonBody(t, map[string]string{
		"refreshToken": "",
	}), jsonHeaders())

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d body=%s", recorder.Code, recorder.Body.String())
	}
	body := decodeAPIResponse(t, recorder)
	if body["code"] != "COMMON-400" || body["message"] != "请求体参数校验失败" {
		t.Fatalf("unexpected body: %#v", body)
	}
	data := body["data"].(map[string]any)
	if data["refreshToken"] != "refreshToken 不能为空" {
		t.Fatalf("unexpected field error: %#v", data)
	}
}

func TestAuthLogoutEndpointRequiresAuthentication(t *testing.T) {
	router, _ := testAuthRouter(t)

	recorder := performRequest(router, http.MethodPost, "/api/v1/auth/logout", nil, nil)

	assertHTTPError(t, recorder, http.StatusUnauthorized, "AUTH-401", "请先登录或重新登录")
}

func TestAuthLogoutEndpointIsNoopForAuthenticatedUser(t *testing.T) {
	router, store := testAuthRouter(t)
	registerViaHTTP(t, router, "alice", "alice@example.com")
	login := loginAndReturnTokenPair(t, router, "alice", "Password123")
	before := store.sessionByID(login.sessionID)

	recorder := performRequest(router, http.MethodPost, "/api/v1/auth/logout", nil, map[string]string{
		"Authorization": "Bearer " + login.accessToken,
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", recorder.Code, recorder.Body.String())
	}
	body := decodeAPIResponse(t, recorder)
	if body["code"] != "COMMON-000" {
		t.Fatalf("unexpected body: %#v", body)
	}
	after := store.sessionByID(login.sessionID)
	if after.Status != before.Status || after.Version != before.Version || after.RefreshTokenHash != before.RefreshTokenHash {
		t.Fatalf("logout should not modify session, before=%#v after=%#v", before, after)
	}
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
	router, store := testAuthRouter(t)
	registerViaHTTP(t, router, "alice", "alice@example.com")
	token := loginAndReturnAccessToken(t, router, "alice", "Password123")
	store.disableUser("alice")

	recorder := performRequest(router, http.MethodGet, "/api/v1/me", nil, map[string]string{
		"Authorization": "Bearer " + token,
	})

	assertHTTPError(t, recorder, http.StatusUnauthorized, "AUTH-401", "请先登录或重新登录")
}

func testAuthRouter(t *testing.T) (http.Handler, *testHTTPAuthStore) {
	t.Helper()
	codec, err := jwt.NewCodec(
		"eventhub-backend",
		"eventhub-test-access-token-secret-for-auth-tests",
		nil,
	)
	if err != nil {
		t.Fatalf("new jwt codec: %v", err)
	}
	store := newTestHTTPAuthStore()
	users := &testHTTPUserRepo{store: store}
	roles := &testHTTPRoleRepo{store: store}
	sessions := &testHTTPSessionRepo{store: store}
	userService := usersvc.NewService(users, roles)
	authService, err := authsvc.NewService(
		users,
		roles,
		sessions,
		testHTTPNoopTransactor{},
		password.NewBCryptHasherWithCost(bcrypt.MinCost),
		codec,
		30*time.Minute,
		refresh.NewManager(30*24*time.Hour),
		userService,
		clock.RealClock{},
	)
	if err != nil {
		t.Fatalf("new auth service: %v", err)
	}
	systemService := systemsvc.NewService(config.Config{AppName: "eventhub-backend", Version: "test"}, clock.RealClock{})
	router := apphttp.NewRouter(testLogger(), apphttp.RouterDependencies{
		System:         systemhandler.NewHandler(systemService),
		Auth:           authhandler.NewHandler(authService),
		User:           userhandler.NewHandler(userService),
		AuthMiddleware: middleware.NewAuth(codec, userService),
	})
	return router, store
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
	return loginAndReturnTokenPair(t, router, usernameOrEmail, password).accessToken
}

type testTokenPair struct {
	accessToken  string
	refreshToken string
	sessionID    string
}

func loginAndReturnTokenPair(t *testing.T, router http.Handler, usernameOrEmail, password string) testTokenPair {
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
	refreshToken, ok := data["refreshToken"].(string)
	if !ok || refreshToken == "" {
		t.Fatalf("missing refreshToken: %#v", data)
	}
	sessionID, ok := data["sessionId"].(string)
	if !ok || sessionID == "" {
		t.Fatalf("missing sessionId: %#v", data)
	}
	return testTokenPair{
		accessToken:  token,
		refreshToken: refreshToken,
		sessionID:    sessionID,
	}
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

type testHTTPNoopTransactor struct{}

func (testHTTPNoopTransactor) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type testHTTPAuthStore struct {
	mu             sync.Mutex
	nextUserID     int64
	nextSessionID  int64
	users          map[int64]repository.User
	username       map[string]int64
	email          map[string]int64
	roles          map[int64]repository.Role
	roleByCode     map[string]int64
	userRoles      map[int64]map[int64]bool
	sessions       map[string]repository.AuthSession
	refreshHashMap map[string]string
}

func newTestHTTPAuthStore() *testHTTPAuthStore {
	return &testHTTPAuthStore{
		nextUserID:     1,
		nextSessionID:  1,
		users:          map[int64]repository.User{},
		username:       map[string]int64{},
		email:          map[string]int64{},
		roles:          map[int64]repository.Role{1: {ID: 1, Code: "USER", Name: "普通用户"}},
		roleByCode:     map[string]int64{"USER": 1},
		userRoles:      map[int64]map[int64]bool{},
		sessions:       map[string]repository.AuthSession{},
		refreshHashMap: map[string]string{},
	}
}

func (s *testHTTPAuthStore) disableUser(username string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if id, ok := s.username[username]; ok {
		user := s.users[id]
		user.Status = repository.UserStatusDisabled
		user.UpdatedAt = time.Now().UTC()
		s.users[id] = user
	}
}

func (s *testHTTPAuthStore) sessionByID(sessionID string) repository.AuthSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[sessionID]
}

type testHTTPUserRepo struct {
	store *testHTTPAuthStore
}

func (r *testHTTPUserRepo) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	_, ok := r.store.username[username]
	return ok, nil
}

func (r *testHTTPUserRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	_, ok := r.store.email[email]
	return ok, nil
}

func (r *testHTTPUserRepo) Create(ctx context.Context, input repository.CreateUserInput) (repository.User, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	if _, ok := r.store.username[input.Username]; ok {
		return repository.User{}, errors.New("duplicate username")
	}
	if _, ok := r.store.email[input.Email]; ok {
		return repository.User{}, errors.New("duplicate email")
	}
	now := time.Now().UTC()
	user := repository.User{
		ID:           r.store.nextUserID,
		Username:     input.Username,
		Email:        input.Email,
		PasswordHash: input.PasswordHash,
		Status:       input.Status,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	r.store.nextUserID++
	r.store.users[user.ID] = user
	r.store.username[user.Username] = user.ID
	r.store.email[user.Email] = user.ID
	return user, nil
}

func (r *testHTTPUserRepo) FindByUsernameOrEmail(ctx context.Context, usernameOrEmail string) (repository.User, bool, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	if id, ok := r.store.username[usernameOrEmail]; ok {
		return r.store.users[id], true, nil
	}
	if id, ok := r.store.email[usernameOrEmail]; ok {
		return r.store.users[id], true, nil
	}
	return repository.User{}, false, nil
}

func (r *testHTTPUserRepo) FindByID(ctx context.Context, id int64) (repository.User, bool, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	user, ok := r.store.users[id]
	return user, ok, nil
}

func (r *testHTTPUserRepo) CountByCriteria(ctx context.Context, criteria repository.UserCriteria) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *testHTTPUserRepo) FindPage(ctx context.Context, criteria repository.UserCriteria, limit int32, offset int32) ([]repository.User, error) {
	return nil, errors.New("not implemented")
}

func (r *testHTTPUserRepo) UpdateStatus(ctx context.Context, id int64, status repository.UserStatus) (int64, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	user, ok := r.store.users[id]
	if !ok {
		return 0, nil
	}
	user.Status = status
	user.UpdatedAt = time.Now().UTC()
	r.store.users[id] = user
	return 1, nil
}

type testHTTPRoleRepo struct {
	store *testHTTPAuthStore
}

func (r *testHTTPRoleRepo) FindByCode(ctx context.Context, code string) (repository.Role, bool, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	id, ok := r.store.roleByCode[code]
	if !ok {
		return repository.Role{}, false, nil
	}
	return r.store.roles[id], true, nil
}

func (r *testHTTPRoleRepo) FindRoleCodesByUserID(ctx context.Context, userID int64) ([]string, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	roleIDs := r.store.userRoles[userID]
	if len(roleIDs) == 0 {
		return []string{}, nil
	}
	codes := make([]string, 0, len(roleIDs))
	for roleID := range roleIDs {
		codes = append(codes, r.store.roles[roleID].Code)
	}
	return codes, nil
}

func (r *testHTTPRoleRepo) FindRoleCodesByUserIDs(ctx context.Context, userIDs []int64) ([]repository.UserRoleCode, error) {
	return nil, errors.New("not implemented")
}

func (r *testHTTPRoleRepo) AddRoleToUser(ctx context.Context, userID, roleID int64) (int64, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	if _, ok := r.store.users[userID]; !ok {
		return 0, nil
	}
	if _, ok := r.store.roles[roleID]; !ok {
		return 0, nil
	}
	if r.store.userRoles[userID] == nil {
		r.store.userRoles[userID] = map[int64]bool{}
	}
	r.store.userRoles[userID][roleID] = true
	return 1, nil
}

type testHTTPSessionRepo struct {
	store *testHTTPAuthStore
}

func (r *testHTTPSessionRepo) Create(ctx context.Context, input repository.CreateAuthSessionInput) (repository.AuthSession, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	if _, ok := r.store.sessions[input.SessionID]; ok {
		return repository.AuthSession{}, errors.New("duplicate session")
	}
	if _, ok := r.store.refreshHashMap[input.RefreshTokenHash]; ok {
		return repository.AuthSession{}, errors.New("duplicate refresh token hash")
	}
	now := time.Now().UTC()
	session := repository.AuthSession{
		ID:               r.store.nextSessionID,
		SessionID:        input.SessionID,
		UserID:           input.UserID,
		RefreshTokenHash: input.RefreshTokenHash,
		Status:           input.Status,
		IssuedAt:         input.IssuedAt,
		RefreshExpiresAt: input.RefreshExpiresAt,
		LastSeenAt:       input.LastSeenAt,
		Version:          input.Version,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	r.store.nextSessionID++
	r.store.sessions[session.SessionID] = session
	r.store.refreshHashMap[session.RefreshTokenHash] = session.SessionID
	return session, nil
}

func (r *testHTTPSessionRepo) FindBySessionID(ctx context.Context, sessionID string) (repository.AuthSession, bool, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	session, ok := r.store.sessions[sessionID]
	return session, ok, nil
}

func (r *testHTTPSessionRepo) FindByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (repository.AuthSession, bool, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	sessionID, ok := r.store.refreshHashMap[refreshTokenHash]
	if !ok {
		return repository.AuthSession{}, false, nil
	}
	return r.store.sessions[sessionID], true, nil
}

func (r *testHTTPSessionRepo) ConditionalRotate(ctx context.Context, input repository.ConditionalRotateAuthSessionInput) (int64, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	session, ok := r.store.sessions[input.SessionID]
	if !ok {
		return 0, nil
	}
	if session.RefreshTokenHash != input.OldRefreshTokenHash ||
		session.Version != input.OldVersion ||
		session.Status != repository.AuthSessionStatusActive ||
		!session.RefreshExpiresAt.After(input.RefreshedAt) {
		return 0, nil
	}
	delete(r.store.refreshHashMap, session.RefreshTokenHash)
	refreshedAt := input.RefreshedAt
	session.RefreshTokenHash = input.NewRefreshTokenHash
	session.RefreshExpiresAt = input.RefreshExpiresAt
	session.LastRefreshedAt = &refreshedAt
	session.LastSeenAt = &refreshedAt
	session.Version++
	session.UpdatedAt = time.Now().UTC()
	r.store.sessions[session.SessionID] = session
	r.store.refreshHashMap[session.RefreshTokenHash] = session.SessionID
	return 1, nil
}

func (r *testHTTPSessionRepo) UpdateLastSeenAt(ctx context.Context, sessionID string, lastSeenAt time.Time) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *testHTTPSessionRepo) RevokeBySessionID(ctx context.Context, input repository.RevokeAuthSessionInput) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *testHTTPSessionRepo) UpdateStatus(ctx context.Context, sessionID string, status repository.AuthSessionStatus) (int64, error) {
	return 0, errors.New("not implemented")
}
