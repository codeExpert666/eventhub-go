package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
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

func TestAdminUsersEndpointRejectsUserRole(t *testing.T) {
	router, _ := testAuthRouter(t)
	registerViaHTTP(t, router, "alice", "alice@example.com")
	token := loginAndReturnAccessToken(t, router, "alice", "Password123")

	recorder := performRequest(router, http.MethodGet, "/api/v1/admin/users", nil, map[string]string{
		"Authorization": "Bearer " + token,
	})

	assertHTTPError(t, recorder, http.StatusForbidden, "AUTH-403", "权限不足")
}

func TestAdminUsersEndpointAllowsAdminAndSupportsPagination(t *testing.T) {
	router, _ := testAuthRouter(t)
	registerViaHTTP(t, router, "pageuser", "pageuser@example.com")
	adminToken := loginAndReturnAccessToken(t, router, "admin", "Admin123456")

	recorder := performRequest(router, http.MethodGet, "/api/v1/admin/users?page=1&size=1", nil, map[string]string{
		"Authorization": "Bearer " + adminToken,
	})

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", recorder.Code, recorder.Body.String())
	}
	data := decodeAPIResponse(t, recorder)["data"].(map[string]any)
	if data["page"] != float64(1) || data["size"] != float64(1) || data["hasNext"] != true || data["hasPrevious"] != false {
		t.Fatalf("unexpected page metadata: %#v", data)
	}
	if data["total"].(float64) <= 1 || data["totalPages"].(float64) <= 1 {
		t.Fatalf("expected multiple users in page metadata: %#v", data)
	}
	items := data["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("expected one item, got %#v", items)
	}
	first := items[0].(map[string]any)
	if first["username"] != "pageuser" {
		t.Fatalf("expected latest registered user first, got %#v", first)
	}
	roles := first["roles"].([]any)
	if len(roles) != 1 || roles[0] != "USER" {
		t.Fatalf("unexpected roles: %#v", roles)
	}
}

func TestAdminUsersEndpointRejectsOversizedPage(t *testing.T) {
	router, _ := testAuthRouter(t)
	adminToken := loginAndReturnAccessToken(t, router, "admin", "Admin123456")

	recorder := performRequest(
		router,
		http.MethodGet,
		"/api/v1/admin/users?page=9223372036854775807&size=100",
		nil,
		map[string]string{"Authorization": "Bearer " + adminToken},
	)

	assertHTTPError(t, recorder, http.StatusBadRequest, "COMMON-400", "请求参数校验失败")
}

func TestAdminUsersEndpointFiltersByUsernameEmailAndStatus(t *testing.T) {
	router, _ := testAuthRouter(t)
	registerViaHTTP(t, router, "filteruser", "filteruser@example.com")
	adminToken := loginAndReturnAccessToken(t, router, "admin", "Admin123456")

	recorder := performRequest(
		router,
		http.MethodGet,
		"/api/v1/admin/users?username=filter&email=FILTERUSER@example.com&status=ENABLED",
		nil,
		map[string]string{"Authorization": "Bearer " + adminToken},
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", recorder.Code, recorder.Body.String())
	}
	data := decodeAPIResponse(t, recorder)["data"].(map[string]any)
	if data["total"] != float64(1) {
		t.Fatalf("expected one filtered user, got %#v", data)
	}
	items := data["items"].([]any)
	user := items[0].(map[string]any)
	if user["username"] != "filteruser" || user["email"] != "filteruser@example.com" || user["status"] != "ENABLED" {
		t.Fatalf("unexpected filtered user: %#v", user)
	}
}

func TestAdminUsersEndpointFiltersByCreatedAtAndUpdatedAtRange(t *testing.T) {
	router, _ := testAuthRouter(t)
	from := time.Now().UTC().Add(-1 * time.Minute).Format("2006-01-02T15:04:05")
	registerViaHTTP(t, router, "timeuser", "timeuser@example.com")
	to := time.Now().UTC().Add(1 * time.Minute).Format("2006-01-02T15:04:05")
	adminToken := loginAndReturnAccessToken(t, router, "admin", "Admin123456")

	recorder := performRequest(
		router,
		http.MethodGet,
		"/api/v1/admin/users?username=timeuser&createdAtFrom="+from+"&createdAtTo="+to+
			"&updatedAtFrom="+from+"&updatedAtTo="+to,
		nil,
		map[string]string{"Authorization": "Bearer " + adminToken},
	)

	if recorder.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", recorder.Code, recorder.Body.String())
	}
	data := decodeAPIResponse(t, recorder)["data"].(map[string]any)
	if data["total"] != float64(1) {
		t.Fatalf("expected one time-range filtered user, got %#v", data)
	}
	items := data["items"].([]any)
	user := items[0].(map[string]any)
	if user["username"] != "timeuser" {
		t.Fatalf("unexpected time-range filtered user: %#v", user)
	}
}

func TestAdminUpdateUserStatusEndpointDisablesOldAccessToken(t *testing.T) {
	router, _ := testAuthRouter(t)
	userID := registerViaHTTPAndReturnUserID(t, router, "target", "target@example.com")
	targetToken := loginAndReturnAccessToken(t, router, "target", "Password123")
	adminToken := loginAndReturnAccessToken(t, router, "admin", "Admin123456")

	updated := performRequest(router, http.MethodPatch, "/api/v1/admin/users/"+
		strconv.FormatInt(userID, 10)+"/status", jsonBody(t, map[string]string{
		"status": "DISABLED",
	}), map[string]string{
		"Authorization": "Bearer " + adminToken,
		"Content-Type":  "application/json",
	})

	if updated.Code != http.StatusOK {
		t.Fatalf("unexpected status: %d body=%s", updated.Code, updated.Body.String())
	}
	data := decodeAPIResponse(t, updated)["data"].(map[string]any)
	if data["status"] != "DISABLED" {
		t.Fatalf("expected disabled user response, got %#v", data)
	}

	me := performRequest(router, http.MethodGet, "/api/v1/me", nil, map[string]string{
		"Authorization": "Bearer " + targetToken,
	})

	assertHTTPError(t, me, http.StatusUnauthorized, "AUTH-401", "请先登录或重新登录")
}

func TestAuthParitySmokeFlow(t *testing.T) {
	router, _ := testAuthRouter(t)
	userID := registerViaHTTPAndReturnUserID(t, router, "smokeuser", "smokeuser@example.com")
	login := loginAndReturnTokenPair(t, router, "smokeuser", "Password123")

	me := performRequest(router, http.MethodGet, "/api/v1/me", nil, map[string]string{
		"Authorization": "Bearer " + login.accessToken,
	})
	if me.Code != http.StatusOK {
		t.Fatalf("me status=%d body=%s", me.Code, me.Body.String())
	}
	meData := decodeAPIResponse(t, me)["data"].(map[string]any)
	if meData["username"] != "smokeuser" || meData["status"] != "ENABLED" {
		t.Fatalf("unexpected me response: %#v", meData)
	}

	refreshResponse := performRequest(router, http.MethodPost, "/api/v1/auth/refresh", jsonBody(t, map[string]string{
		"refreshToken": login.refreshToken,
	}), jsonHeaders())
	if refreshResponse.Code != http.StatusOK {
		t.Fatalf("refresh status=%d body=%s", refreshResponse.Code, refreshResponse.Body.String())
	}
	refreshData := decodeAPIResponse(t, refreshResponse)["data"].(map[string]any)
	if refreshData["sessionId"] != login.sessionID {
		t.Fatalf("expected same session id after refresh, got %#v", refreshData)
	}
	if refreshData["refreshToken"] == "" || refreshData["refreshToken"] == login.refreshToken {
		t.Fatalf("expected rotated refresh token, got %#v", refreshData)
	}

	replay := performRequest(router, http.MethodPost, "/api/v1/auth/refresh", jsonBody(t, map[string]string{
		"refreshToken": login.refreshToken,
	}), jsonHeaders())
	assertHTTPError(t, replay, http.StatusUnauthorized, "AUTH-401", "refresh token 无效或已过期")

	refreshedAccessToken := refreshData["accessToken"].(string)
	logout := performRequest(router, http.MethodPost, "/api/v1/auth/logout", nil, map[string]string{
		"Authorization": "Bearer " + refreshedAccessToken,
	})
	if logout.Code != http.StatusOK {
		t.Fatalf("logout status=%d body=%s", logout.Code, logout.Body.String())
	}
	logoutBody := decodeAPIResponse(t, logout)
	if logoutBody["code"] != "COMMON-000" || logoutBody["data"] != nil {
		t.Fatalf("unexpected logout body: %#v", logoutBody)
	}

	adminToken := loginAndReturnAccessToken(t, router, "admin", "Admin123456")
	adminList := performRequest(router, http.MethodGet, "/api/v1/admin/users?username=smokeuser", nil, map[string]string{
		"Authorization": "Bearer " + adminToken,
	})
	if adminList.Code != http.StatusOK {
		t.Fatalf("admin list status=%d body=%s", adminList.Code, adminList.Body.String())
	}
	listData := decodeAPIResponse(t, adminList)["data"].(map[string]any)
	if listData["total"] != float64(1) {
		t.Fatalf("expected one smoke user, got %#v", listData)
	}
	items := listData["items"].([]any)
	item := items[0].(map[string]any)
	if item["id"] != float64(userID) || item["username"] != "smokeuser" {
		t.Fatalf("unexpected admin list item: %#v", item)
	}

	disable := performRequest(router, http.MethodPatch, "/api/v1/admin/users/"+
		strconv.FormatInt(userID, 10)+"/status", jsonBody(t, map[string]string{
		"status": "DISABLED",
	}), map[string]string{
		"Authorization": "Bearer " + adminToken,
		"Content-Type":  "application/json",
	})
	if disable.Code != http.StatusOK {
		t.Fatalf("disable status=%d body=%s", disable.Code, disable.Body.String())
	}
	disableData := decodeAPIResponse(t, disable)["data"].(map[string]any)
	if disableData["status"] != "DISABLED" {
		t.Fatalf("expected disabled status, got %#v", disableData)
	}

	oldTokenMe := performRequest(router, http.MethodGet, "/api/v1/me", nil, map[string]string{
		"Authorization": "Bearer " + login.accessToken,
	})
	assertHTTPError(t, oldTokenMe, http.StatusUnauthorized, "AUTH-401", "请先登录或重新登录")
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
	store.seedUser(t, "admin", "admin@eventhub.local", "Admin123456", []string{"USER", "ADMIN"})
	users := &testHTTPUserRepo{store: store}
	roles := &testHTTPRoleRepo{store: store}
	sessions := &testHTTPSessionRepo{store: store}
	userService := usersvc.NewService(users, roles, testHTTPNoopTransactor{})
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
	_ = registerViaHTTPAndReturnUserID(t, router, username, email)
}

func registerViaHTTPAndReturnUserID(t *testing.T, router http.Handler, username, email string) int64 {
	t.Helper()
	recorder := performRequest(router, http.MethodPost, "/api/v1/auth/register", jsonBody(t, map[string]string{
		"username": username,
		"email":    email,
		"password": "Password123",
	}), jsonHeaders())
	if recorder.Code != http.StatusOK {
		t.Fatalf("register status=%d body=%s", recorder.Code, recorder.Body.String())
	}
	data := decodeAPIResponse(t, recorder)["data"].(map[string]any)
	return int64(data["id"].(float64))
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
		nextUserID:    1,
		nextSessionID: 1,
		users:         map[int64]repository.User{},
		username:      map[string]int64{},
		email:         map[string]int64{},
		roles: map[int64]repository.Role{
			1: {ID: 1, Code: "USER", Name: "普通用户"},
			2: {ID: 2, Code: "ADMIN", Name: "管理员"},
		},
		roleByCode:     map[string]int64{"USER": 1, "ADMIN": 2},
		userRoles:      map[int64]map[int64]bool{},
		sessions:       map[string]repository.AuthSession{},
		refreshHashMap: map[string]string{},
	}
}

func (s *testHTTPAuthStore) seedUser(t *testing.T, username, email, rawPassword string, roleCodes []string) int64 {
	t.Helper()
	hash, err := bcrypt.GenerateFromPassword([]byte(rawPassword), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hash seed password: %v", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	user := repository.User{
		ID:           s.nextUserID,
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		Status:       repository.UserStatusEnabled,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.nextUserID++
	s.users[user.ID] = user
	s.username[user.Username] = user.ID
	s.email[user.Email] = user.ID
	s.userRoles[user.ID] = map[int64]bool{}
	for _, code := range roleCodes {
		roleID, ok := s.roleByCode[code]
		if !ok {
			t.Fatalf("unknown seed role: %s", code)
		}
		s.userRoles[user.ID][roleID] = true
	}
	return user.ID
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
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	var count int64
	for _, user := range r.store.users {
		if matchesUserCriteria(user, criteria) {
			count++
		}
	}
	return count, nil
}

func (r *testHTTPUserRepo) ListUsers(ctx context.Context, criteria repository.UserCriteria, limit int32, offset int32) ([]repository.User, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	users := make([]repository.User, 0, len(r.store.users))
	for _, user := range r.store.users {
		if matchesUserCriteria(user, criteria) {
			users = append(users, user)
		}
	}
	sort.Slice(users, func(i, j int) bool {
		if users[i].CreatedAt.Equal(users[j].CreatedAt) {
			return users[i].ID > users[j].ID
		}
		return users[i].CreatedAt.After(users[j].CreatedAt)
	})
	if offset >= int32(len(users)) {
		return []repository.User{}, nil
	}
	end := offset + limit
	if end > int32(len(users)) {
		end = int32(len(users))
	}
	page := make([]repository.User, end-offset)
	copy(page, users[offset:end])
	return page, nil
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

func matchesUserCriteria(user repository.User, criteria repository.UserCriteria) bool {
	if criteria.Username != "" && !strings.Contains(user.Username, criteria.Username) {
		return false
	}
	if criteria.Email != "" && !strings.Contains(user.Email, criteria.Email) {
		return false
	}
	if criteria.Status != nil && user.Status != *criteria.Status {
		return false
	}
	if criteria.CreatedAtFrom != nil && user.CreatedAt.Before(*criteria.CreatedAtFrom) {
		return false
	}
	if criteria.CreatedAtTo != nil && user.CreatedAt.After(*criteria.CreatedAtTo) {
		return false
	}
	if criteria.UpdatedAtFrom != nil && user.UpdatedAt.Before(*criteria.UpdatedAtFrom) {
		return false
	}
	if criteria.UpdatedAtTo != nil && user.UpdatedAt.After(*criteria.UpdatedAtTo) {
		return false
	}
	return true
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
	sort.Strings(codes)
	return codes, nil
}

func (r *testHTTPRoleRepo) FindRoleCodesByUserIDs(ctx context.Context, userIDs []int64) ([]repository.UserRoleCode, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	rows := make([]repository.UserRoleCode, 0)
	for _, userID := range userIDs {
		roleIDs := r.store.userRoles[userID]
		if len(roleIDs) == 0 {
			continue
		}
		codes := make([]string, 0, len(roleIDs))
		for roleID := range roleIDs {
			codes = append(codes, r.store.roles[roleID].Code)
		}
		sort.Strings(codes)
		for _, code := range codes {
			rows = append(rows, repository.UserRoleCode{
				UserID:   userID,
				RoleCode: code,
			})
		}
	}
	return rows, nil
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
