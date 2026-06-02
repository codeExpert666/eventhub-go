package auth

import (
	"context"
	"errors"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	drivermysql "github.com/go-sql-driver/mysql"
	"golang.org/x/crypto/bcrypt"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/repository"
	"eventhub-go/internal/security/jwt"
	"eventhub-go/internal/security/password"
	"eventhub-go/internal/security/refresh"
	usersvc "eventhub-go/internal/service/user"
)

func TestRegisterCreatesEnabledUserWithDefaultRole(t *testing.T) {
	ctx := context.Background()
	fixture := newAuthServiceFixture(t)

	user, err := fixture.service.Register(ctx, RegisterCommand{
		Username: "alice",
		Email:    "ALICE@example.com",
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	if user.Username != "alice" || user.Email != "alice@example.com" || user.Status != "ENABLED" {
		t.Fatalf("unexpected user: %#v", user)
	}
	if len(user.Roles) != 1 || user.Roles[0] != "USER" {
		t.Fatalf("expected USER role, got %v", user.Roles)
	}
	stored := fixture.store.userByID(user.ID)
	if stored.PasswordHash == "Password123" || !strings.HasPrefix(stored.PasswordHash, "$2") {
		t.Fatalf("unexpected password hash: %s", stored.PasswordHash)
	}
	matches, err := fixture.service.passwords.Matches("Password123", stored.PasswordHash)
	if err != nil {
		t.Fatalf("match password: %v", err)
	}
	if !matches {
		t.Fatal("expected stored password hash to match original password")
	}
}

func TestRegisterRejectsDuplicateUsername(t *testing.T) {
	ctx := context.Background()
	fixture := newAuthServiceFixture(t)
	if _, err := fixture.service.Register(ctx, RegisterCommand{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "Password123",
	}); err != nil {
		t.Fatalf("seed register: %v", err)
	}

	_, err := fixture.service.Register(ctx, RegisterCommand{
		Username: "alice",
		Email:    "alice2@example.com",
		Password: "Password123",
	})

	assertAppError(t, err, apperror.AuthConflict, "用户名已存在")
}

func TestRegisterMapsDatabaseUniqueConstraintToAuthConflict(t *testing.T) {
	ctx := context.Background()
	fixture := newAuthServiceFixture(t)
	fixture.store.forceCreateUniqueErr = true

	_, err := fixture.service.Register(ctx, RegisterCommand{
		Username: "race",
		Email:    "race@example.com",
		Password: "Password123",
	})

	assertAppError(t, err, apperror.AuthConflict, "用户名或邮箱已存在")
}

func TestLoginCreatesActiveSessionAndReturnsTokenPair(t *testing.T) {
	ctx := context.Background()
	fixture := newAuthServiceFixture(t)
	registered, err := fixture.service.Register(ctx, RegisterCommand{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	login, err := fixture.service.Login(ctx, LoginCommand{
		UsernameOrEmail: "alice",
		Password:        "Password123",
	})
	if err != nil {
		t.Fatalf("login: %v", err)
	}

	if login.AccessToken == "" || login.RefreshToken == "" || login.SessionID == "" {
		t.Fatalf("expected token pair and session id: %#v", login)
	}
	if login.AuthorizationScheme != "Bearer" {
		t.Fatalf("unexpected scheme: %s", login.AuthorizationScheme)
	}
	if login.ExpiresIn != 1800 || login.RefreshExpiresIn != 2592000 {
		t.Fatalf("unexpected ttls: access=%d refresh=%d", login.ExpiresIn, login.RefreshExpiresIn)
	}
	if login.User.ID != registered.ID || login.User.Username != "alice" {
		t.Fatalf("unexpected login user: %#v", login.User)
	}

	session := fixture.store.sessionByID(login.SessionID)
	if session.UserID != registered.ID || session.Status != repository.AuthSessionStatusActive {
		t.Fatalf("unexpected session: %#v", session)
	}
	if session.RefreshTokenHash == login.RefreshToken || !strings.HasPrefix(session.RefreshTokenHash, "sha256:") {
		t.Fatalf("refresh token hash was not stored correctly: %s", session.RefreshTokenHash)
	}
}

func TestLoginRejectsWrongPasswordWithoutCreatingSession(t *testing.T) {
	ctx := context.Background()
	fixture := newAuthServiceFixture(t)
	if _, err := fixture.service.Register(ctx, RegisterCommand{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "Password123",
	}); err != nil {
		t.Fatalf("register: %v", err)
	}

	_, err := fixture.service.Login(ctx, LoginCommand{
		UsernameOrEmail: "alice",
		Password:        "WrongPassword123",
	})

	assertAppError(t, err, apperror.AuthUnauthorized, "账号或密码错误")
	if got := fixture.store.sessionCount(); got != 0 {
		t.Fatalf("expected no session, got %d", got)
	}
}

func TestLoginRejectsDisabledUser(t *testing.T) {
	ctx := context.Background()
	fixture := newAuthServiceFixture(t)
	user, err := fixture.service.Register(ctx, RegisterCommand{
		Username: "alice",
		Email:    "alice@example.com",
		Password: "Password123",
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, err := fixture.users.UpdateStatus(ctx, user.ID, repository.UserStatusDisabled); err != nil {
		t.Fatalf("disable user: %v", err)
	}

	_, err = fixture.service.Login(ctx, LoginCommand{
		UsernameOrEmail: "alice",
		Password:        "Password123",
	})

	assertAppError(t, err, apperror.AuthForbidden, "用户已被禁用")
	if got := fixture.store.sessionCount(); got != 0 {
		t.Fatalf("expected no session, got %d", got)
	}
}

type authServiceFixture struct {
	store    *authServiceTestStore
	users    *authServiceUserRepo
	roles    *authServiceRoleRepo
	sessions *authServiceSessionRepo
	service  *Service
}

func newAuthServiceFixture(t *testing.T) authServiceFixture {
	t.Helper()
	store := newAuthServiceTestStore()
	users := &authServiceUserRepo{store: store}
	roles := &authServiceRoleRepo{store: store}
	sessions := &authServiceSessionRepo{store: store}
	userService := usersvc.NewService(users, roles)
	codec, err := jwt.NewCodec(jwt.Config{
		Issuer:        "eventhub-backend",
		SigningSecret: "eventhub-test-access-token-secret-for-service-tests",
		AccessTTL:     30 * time.Minute,
	})
	if err != nil {
		t.Fatalf("new jwt codec: %v", err)
	}
	service := NewService(Dependencies{
		Users:        users,
		Roles:        roles,
		Sessions:     sessions,
		Transactor:   noopTransactor{},
		Passwords:    password.NewBCryptHasherWithCost(bcrypt.MinCost),
		Tokens:       codec,
		RefreshToken: refresh.NewManager(30 * 24 * time.Hour),
		UserService:  userService,
		Clock:        testClock{now: time.Date(2026, 6, 2, 12, 0, 0, 0, time.UTC)},
	})
	return authServiceFixture{
		store:    store,
		users:    users,
		roles:    roles,
		sessions: sessions,
		service:  service,
	}
}

type noopTransactor struct{}

func (noopTransactor) WithinTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

type testClock struct {
	now time.Time
}

func (c testClock) Now() time.Time {
	return c.now
}

type authServiceTestStore struct {
	mu                   sync.Mutex
	nextUserID           int64
	nextSessionID        int64
	users                map[int64]repository.User
	userByUsername       map[string]int64
	userByEmail          map[string]int64
	roles                map[int64]repository.Role
	roleByCode           map[string]int64
	userRoles            map[int64]map[int64]bool
	sessions             map[string]repository.AuthSession
	sessionByRefreshHash map[string]string
	forceCreateUniqueErr bool
}

func newAuthServiceTestStore() *authServiceTestStore {
	return &authServiceTestStore{
		nextUserID:           1,
		nextSessionID:        1,
		users:                map[int64]repository.User{},
		userByUsername:       map[string]int64{},
		userByEmail:          map[string]int64{},
		roles:                map[int64]repository.Role{1: {ID: 1, Code: "USER", Name: "普通用户"}},
		roleByCode:           map[string]int64{"USER": 1},
		userRoles:            map[int64]map[int64]bool{},
		sessions:             map[string]repository.AuthSession{},
		sessionByRefreshHash: map[string]string{},
	}
}

type authServiceUserRepo struct {
	store *authServiceTestStore
}

func (r *authServiceUserRepo) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	_, ok := r.store.userByUsername[username]
	return ok, nil
}

func (r *authServiceUserRepo) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	_, ok := r.store.userByEmail[email]
	return ok, nil
}

func (r *authServiceUserRepo) Create(ctx context.Context, input repository.CreateUserInput) (repository.User, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	if r.store.forceCreateUniqueErr {
		return repository.User{}, &drivermysql.MySQLError{Number: 1062, Message: "duplicate"}
	}
	if _, ok := r.store.userByUsername[input.Username]; ok {
		return repository.User{}, &drivermysql.MySQLError{Number: 1062, Message: "duplicate username"}
	}
	if _, ok := r.store.userByEmail[input.Email]; ok {
		return repository.User{}, &drivermysql.MySQLError{Number: 1062, Message: "duplicate email"}
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
	r.store.userByUsername[user.Username] = user.ID
	r.store.userByEmail[user.Email] = user.ID
	return user, nil
}

func (r *authServiceUserRepo) FindByUsernameOrEmail(ctx context.Context, usernameOrEmail string) (repository.User, bool, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	if id, ok := r.store.userByUsername[usernameOrEmail]; ok {
		return r.store.users[id], true, nil
	}
	if id, ok := r.store.userByEmail[usernameOrEmail]; ok {
		return r.store.users[id], true, nil
	}
	return repository.User{}, false, nil
}

func (r *authServiceUserRepo) FindByID(ctx context.Context, id int64) (repository.User, bool, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	user, ok := r.store.users[id]
	return user, ok, nil
}

func (r *authServiceUserRepo) CountByCriteria(ctx context.Context, criteria repository.UserCriteria) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *authServiceUserRepo) FindPage(ctx context.Context, criteria repository.UserCriteria, limit int32, offset int32) ([]repository.User, error) {
	return nil, errors.New("not implemented")
}

func (r *authServiceUserRepo) UpdateStatus(ctx context.Context, id int64, status repository.UserStatus) (int64, error) {
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

type authServiceRoleRepo struct {
	store *authServiceTestStore
}

func (r *authServiceRoleRepo) FindByCode(ctx context.Context, code string) (repository.Role, bool, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	id, ok := r.store.roleByCode[code]
	if !ok {
		return repository.Role{}, false, nil
	}
	return r.store.roles[id], true, nil
}

func (r *authServiceRoleRepo) FindRoleCodesByUserID(ctx context.Context, userID int64) ([]string, error) {
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

func (r *authServiceRoleRepo) FindRoleCodesByUserIDs(ctx context.Context, userIDs []int64) ([]repository.UserRoleCode, error) {
	return nil, errors.New("not implemented")
}

func (r *authServiceRoleRepo) AddRoleToUser(ctx context.Context, userID, roleID int64) (int64, error) {
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

type authServiceSessionRepo struct {
	store *authServiceTestStore
}

func (r *authServiceSessionRepo) Create(ctx context.Context, input repository.CreateAuthSessionInput) (repository.AuthSession, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
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
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
	}
	r.store.nextSessionID++
	r.store.sessions[session.SessionID] = session
	r.store.sessionByRefreshHash[session.RefreshTokenHash] = session.SessionID
	return session, nil
}

func (r *authServiceSessionRepo) FindBySessionID(ctx context.Context, sessionID string) (repository.AuthSession, bool, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	session, ok := r.store.sessions[sessionID]
	return session, ok, nil
}

func (r *authServiceSessionRepo) FindByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (repository.AuthSession, bool, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	sessionID, ok := r.store.sessionByRefreshHash[refreshTokenHash]
	if !ok {
		return repository.AuthSession{}, false, nil
	}
	return r.store.sessions[sessionID], true, nil
}

func (r *authServiceSessionRepo) RotateRefreshToken(ctx context.Context, input repository.RotateRefreshTokenInput) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *authServiceSessionRepo) UpdateLastSeenAt(ctx context.Context, sessionID string, lastSeenAt time.Time) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *authServiceSessionRepo) RevokeBySessionID(ctx context.Context, input repository.RevokeAuthSessionInput) (int64, error) {
	return 0, errors.New("not implemented")
}

func (r *authServiceSessionRepo) UpdateStatus(ctx context.Context, sessionID string, status repository.AuthSessionStatus) (int64, error) {
	r.store.mu.Lock()
	defer r.store.mu.Unlock()
	session, ok := r.store.sessions[sessionID]
	if !ok {
		return 0, nil
	}
	session.Status = status
	r.store.sessions[sessionID] = session
	return 1, nil
}

func (s *authServiceTestStore) userByID(id int64) repository.User {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.users[id]
}

func (s *authServiceTestStore) sessionByID(sessionID string) repository.AuthSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sessions[sessionID]
}

func (s *authServiceTestStore) sessionCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions)
}

func assertAppError(t *testing.T, err error, code apperror.Code, message string) {
	t.Helper()
	if err == nil {
		t.Fatal("expected error")
	}
	appErr, ok := apperror.FromError(err)
	if !ok {
		t.Fatalf("expected AppError, got %T %v", err, err)
	}
	if appErr.Code() != code || appErr.Message() != message {
		t.Fatalf("unexpected app error: code=%s message=%s", appErr.Code().String(), appErr.Message())
	}
}
