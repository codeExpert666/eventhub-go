package mysql_test

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratemysql "github.com/golang-migrate/migrate/v4/database/mysql"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/testcontainers/testcontainers-go"
	tcmysql "github.com/testcontainers/testcontainers-go/modules/mysql"

	platformdb "eventhub-go/internal/platform/db"
	"eventhub-go/internal/repository"
	repositorymysql "eventhub-go/internal/repository/mysql"
)

const testDatabaseName = "eventhub_test"

func TestMySQLPersistenceFoundation(t *testing.T) {
	testcontainers.SkipIfProviderIsNotHealthy(t)

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	container, err := tcmysql.Run(
		ctx,
		"mysql:8.0.36",
		tcmysql.WithDatabase(testDatabaseName),
		tcmysql.WithUsername("eventhub"),
		tcmysql.WithPassword("eventhub"),
	)
	if err != nil {
		t.Fatalf("start mysql container: %v", err)
	}
	t.Cleanup(func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			t.Logf("terminate mysql container: %v", err)
		}
	})

	dsn, err := container.ConnectionString(ctx, "parseTime=true", "multiStatements=true")
	if err != nil {
		t.Fatalf("mysql connection string: %v", err)
	}
	database, err := platformdb.OpenMySQL(ctx, platformdb.Config{
		DSN:          dsn,
		MaxOpenConns: 8,
		MaxIdleConns: 4,
	})
	if err != nil {
		t.Fatalf("open mysql: %v", err)
	}
	t.Cleanup(func() {
		if err := database.Close(); err != nil {
			t.Logf("close mysql: %v", err)
		}
	})

	runMigrations(t, ctx, database, func(m *migrate.Migrate) error {
		return m.Up()
	})
	assertSchemaBootstrapped(t, ctx, database)

	runMigrations(t, ctx, database, func(m *migrate.Migrate) error {
		return m.Down()
	})
	if tableExists(t, ctx, database, "users") {
		t.Fatal("expected users table to be dropped after migration down")
	}

	runMigrations(t, ctx, database, func(m *migrate.Migrate) error {
		return m.Up()
	})
	assertRepositoryBehavior(t, ctx, database)
}

func runMigrations(
	t *testing.T,
	ctx context.Context,
	database *sql.DB,
	action func(*migrate.Migrate) error,
) {
	t.Helper()

	conn, err := database.Conn(ctx)
	if err != nil {
		t.Fatalf("migration connection: %v", err)
	}
	driver, err := migratemysql.WithConnection(ctx, conn, &migratemysql.Config{
		DatabaseName: testDatabaseName,
	})
	if err != nil {
		_ = conn.Close()
		t.Fatalf("migration driver: %v", err)
	}
	migrator, err := migrate.NewWithDatabaseInstance(migrationsSourceURL(t), "mysql", driver)
	if err != nil {
		_ = conn.Close()
		t.Fatalf("migration instance: %v", err)
	}
	if err := action(migrator); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		_, _ = migrator.Close()
		t.Fatalf("run migration: %v", err)
	}
	if sourceErr, databaseErr := migrator.Close(); sourceErr != nil || databaseErr != nil {
		t.Fatalf("close migration: source=%v database=%v", sourceErr, databaseErr)
	}
}

func migrationsSourceURL(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test file path")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "..", "migrations")
	return (&url.URL{Scheme: "file", Path: path}).String()
}

func assertSchemaBootstrapped(t *testing.T, ctx context.Context, database *sql.DB) {
	t.Helper()

	for _, table := range []string{
		"system_bootstrap_record",
		"users",
		"roles",
		"user_roles",
		"auth_sessions",
	} {
		if !tableExists(t, ctx, database, table) {
			t.Fatalf("expected table %s to exist", table)
		}
	}

	var roleCount int
	if err := database.QueryRowContext(
		ctx,
		"SELECT COUNT(*) FROM roles WHERE code IN ('USER', 'ADMIN')",
	).Scan(&roleCount); err != nil {
		t.Fatalf("count seed roles: %v", err)
	}
	if roleCount != 2 {
		t.Fatalf("expected USER and ADMIN seed roles, got %d", roleCount)
	}

	var adminRoleCount int
	if err := database.QueryRowContext(
		ctx,
		`SELECT COUNT(*)
		 FROM user_roles ur
		 JOIN users u ON u.id = ur.user_id
		 JOIN roles r ON r.id = ur.role_id
		 WHERE u.username = 'admin'
		   AND r.code IN ('USER', 'ADMIN')`,
	).Scan(&adminRoleCount); err != nil {
		t.Fatalf("count admin roles: %v", err)
	}
	if adminRoleCount != 2 {
		t.Fatalf("expected demo admin to have USER and ADMIN, got %d roles", adminRoleCount)
	}
}

func tableExists(t *testing.T, ctx context.Context, database *sql.DB, table string) bool {
	t.Helper()

	var count int
	if err := database.QueryRowContext(
		ctx,
		`SELECT COUNT(*)
		 FROM information_schema.tables
		 WHERE table_schema = DATABASE()
		   AND table_name = ?`,
		table,
	).Scan(&count); err != nil {
		t.Fatalf("check table %s: %v", table, err)
	}
	return count > 0
}

func assertRepositoryBehavior(t *testing.T, ctx context.Context, database *sql.DB) {
	t.Helper()

	userRepo := repositorymysql.NewUserRepository(database)
	roleRepo := repositorymysql.NewRoleRepository(database)
	sessionRepo := repositorymysql.NewAuthSessionRepository(database)
	transactor := platformdb.NewTransactor(database, nil)

	userRole, found, err := roleRepo.FindByCode(ctx, "USER")
	if err != nil {
		t.Fatalf("find USER role: %v", err)
	}
	if !found {
		t.Fatal("expected USER seed role")
	}
	adminRole, found, err := roleRepo.FindByCode(ctx, "ADMIN")
	if err != nil {
		t.Fatalf("find ADMIN role: %v", err)
	}
	if !found {
		t.Fatal("expected ADMIN seed role")
	}
	if adminRole.Code != "ADMIN" || userRole.Code != "USER" {
		t.Fatalf("unexpected role codes: %s %s", adminRole.Code, userRole.Code)
	}

	var created repository.User
	if err := transactor.WithinTx(ctx, func(txCtx context.Context) error {
		var err error
		created, err = userRepo.Create(txCtx, repository.CreateUserInput{
			Username:     "alice",
			Email:        "alice@eventhub.test",
			PasswordHash: "$2y$10$test-password-hash-placeholder",
			Status:       repository.UserStatusEnabled,
		})
		if err != nil {
			return err
		}
		rows, err := roleRepo.AddRoleToUser(txCtx, created.ID, userRole.ID)
		if err != nil {
			return err
		}
		if rows != 1 {
			return fmt.Errorf("expected one user role row, got %d", rows)
		}
		return nil
	}); err != nil {
		t.Fatalf("create user in transaction: %v", err)
	}

	exists, err := userRepo.ExistsByUsername(ctx, "alice")
	if err != nil {
		t.Fatalf("exists by username: %v", err)
	}
	if !exists {
		t.Fatal("expected alice username to exist")
	}
	foundUser, found, err := userRepo.FindByUsernameOrEmail(ctx, "alice@eventhub.test")
	if err != nil {
		t.Fatalf("find by username or email: %v", err)
	}
	if !found || foundUser.ID != created.ID {
		t.Fatalf("expected to find created user, found=%v id=%d", found, foundUser.ID)
	}

	codes, err := roleRepo.FindRoleCodesByUserID(ctx, created.ID)
	if err != nil {
		t.Fatalf("find role codes by user id: %v", err)
	}
	if len(codes) != 1 || codes[0] != "USER" {
		t.Fatalf("expected alice to have USER role, got %v", codes)
	}
	codes, err = roleRepo.FindRoleCodesByUserID(ctx, created.ID+999999)
	if err != nil {
		t.Fatalf("find role codes by missing user id: %v", err)
	}
	if codes == nil || len(codes) != 0 {
		t.Fatalf("expected missing user roles to be an empty slice, got %#v", codes)
	}

	roleRows, err := roleRepo.FindRoleCodesByUserIDs(ctx, []int64{created.ID})
	if err != nil {
		t.Fatalf("find role codes by user ids: %v", err)
	}
	if len(roleRows) != 1 || roleRows[0].RoleCode != "USER" {
		t.Fatalf("expected one USER role row, got %v", roleRows)
	}

	enabled := repository.UserStatusEnabled
	count, err := userRepo.CountByCriteria(ctx, repository.UserCriteria{
		Username: "ali",
		Status:   &enabled,
	})
	if err != nil {
		t.Fatalf("count users by criteria: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one enabled ali user, got %d", count)
	}
	page, err := userRepo.FindPage(ctx, repository.UserCriteria{Status: &enabled}, 10, 0)
	if err != nil {
		t.Fatalf("find users page: %v", err)
	}
	if len(page) < 2 {
		t.Fatalf("expected seeded admin and created user in page, got %d", len(page))
	}

	if _, err := userRepo.Create(ctx, repository.CreateUserInput{
		Username:     "alice",
		Email:        "alice-duplicate@eventhub.test",
		PasswordHash: "$2y$10$test-password-hash-placeholder",
		Status:       repository.UserStatusEnabled,
	}); !platformdb.IsUniqueConstraintError(err) {
		t.Fatalf("expected duplicate username to be recognized as unique constraint error, got %v", err)
	}

	assertAuthSessionRepositoryBehavior(t, ctx, created.ID, sessionRepo)
}

func assertAuthSessionRepositoryBehavior(
	t *testing.T,
	ctx context.Context,
	userID int64,
	sessionRepo *repositorymysql.AuthSessionRepository,
) {
	t.Helper()

	issuedAt := time.Now().UTC().Truncate(time.Second)
	lastSeenAt := issuedAt
	clientIPHash := "hash-ip"
	userAgentHash := "hash-ua"
	userAgentSummary := "Go integration test"
	session, err := sessionRepo.Create(ctx, repository.CreateAuthSessionInput{
		SessionID:        "sess-integration",
		UserID:           userID,
		RefreshTokenHash: "hash-old-refresh-token",
		Status:           repository.AuthSessionStatusActive,
		IssuedAt:         issuedAt,
		RefreshExpiresAt: issuedAt.Add(30 * 24 * time.Hour),
		LastSeenAt:       &lastSeenAt,
		ClientIPHash:     &clientIPHash,
		UserAgentHash:    &userAgentHash,
		UserAgentSummary: &userAgentSummary,
	})
	if err != nil {
		t.Fatalf("create auth session: %v", err)
	}
	if session.Status != repository.AuthSessionStatusActive || session.Version != 0 {
		t.Fatalf("unexpected session status/version: %s %d", session.Status, session.Version)
	}
	if session.LastSeenAt == nil || !session.LastSeenAt.Equal(lastSeenAt) {
		t.Fatalf("expected last_seen_at %v, got %v", lastSeenAt, session.LastSeenAt)
	}

	foundByHash, found, err := sessionRepo.FindByRefreshTokenHash(ctx, "hash-old-refresh-token")
	if err != nil {
		t.Fatalf("find by refresh token hash: %v", err)
	}
	if !found || foundByHash.SessionID != session.SessionID {
		t.Fatalf("expected to find session by refresh hash, found=%v session=%s", found, foundByHash.SessionID)
	}

	if _, err := sessionRepo.Create(ctx, repository.CreateAuthSessionInput{
		SessionID:        "sess-integration",
		UserID:           userID,
		RefreshTokenHash: "hash-another-token",
		Status:           repository.AuthSessionStatusActive,
		IssuedAt:         issuedAt,
		RefreshExpiresAt: issuedAt.Add(30 * 24 * time.Hour),
	}); !platformdb.IsUniqueConstraintError(err) {
		t.Fatalf("expected duplicate session_id unique constraint error, got %v", err)
	}

	refreshedAt := issuedAt.Add(time.Hour)
	rows, err := sessionRepo.ConditionalRotate(ctx, repository.ConditionalRotateAuthSessionInput{
		SessionID:           session.SessionID,
		OldRefreshTokenHash: "hash-old-refresh-token",
		OldVersion:          session.Version,
		NewRefreshTokenHash: "hash-new-refresh-token",
		RefreshedAt:         refreshedAt,
		RefreshExpiresAt:    refreshedAt.Add(30 * 24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("rotate refresh token: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected first rotate to affect 1 row, got %d", rows)
	}
	rows, err = sessionRepo.ConditionalRotate(ctx, repository.ConditionalRotateAuthSessionInput{
		SessionID:           session.SessionID,
		OldRefreshTokenHash: "hash-old-refresh-token",
		OldVersion:          session.Version,
		NewRefreshTokenHash: "hash-new-refresh-token-2",
		RefreshedAt:         refreshedAt,
		RefreshExpiresAt:    refreshedAt.Add(30 * 24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("rotate refresh token second time: %v", err)
	}
	if rows != 0 {
		t.Fatalf("expected second rotate with stale token/version to affect 0 rows, got %d", rows)
	}

	rotated, found, err := sessionRepo.FindBySessionID(ctx, session.SessionID)
	if err != nil {
		t.Fatalf("find rotated session: %v", err)
	}
	if !found {
		t.Fatal("expected rotated session to exist")
	}
	if rotated.RefreshTokenHash != "hash-new-refresh-token" || rotated.Version != 1 {
		t.Fatalf("unexpected rotated hash/version: %s %d", rotated.RefreshTokenHash, rotated.Version)
	}

	seenAt := refreshedAt.Add(time.Hour)
	rows, err = sessionRepo.UpdateLastSeenAt(ctx, session.SessionID, seenAt)
	if err != nil {
		t.Fatalf("update last seen: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected last seen update to affect 1 row, got %d", rows)
	}

	revokedAt := seenAt.Add(time.Hour)
	rows, err = sessionRepo.RevokeBySessionID(ctx, repository.RevokeAuthSessionInput{
		SessionID:    session.SessionID,
		RevokedAt:    revokedAt,
		RevokeReason: "LOGOUT",
	})
	if err != nil {
		t.Fatalf("revoke session: %v", err)
	}
	if rows != 1 {
		t.Fatalf("expected revoke to affect 1 row, got %d", rows)
	}
	rows, err = sessionRepo.RevokeBySessionID(ctx, repository.RevokeAuthSessionInput{
		SessionID:    session.SessionID,
		RevokedAt:    revokedAt,
		RevokeReason: "LOGOUT",
	})
	if err != nil {
		t.Fatalf("revoke session second time: %v", err)
	}
	if rows != 0 {
		t.Fatalf("expected second revoke to affect 0 rows, got %d", rows)
	}

	revoked, found, err := sessionRepo.FindBySessionID(ctx, session.SessionID)
	if err != nil {
		t.Fatalf("find revoked session: %v", err)
	}
	if !found {
		t.Fatal("expected revoked session to exist")
	}
	if revoked.Status != repository.AuthSessionStatusRevoked {
		t.Fatalf("expected revoked session status, got %s", revoked.Status)
	}
	if revoked.RevokeReason == nil || *revoked.RevokeReason != "LOGOUT" {
		t.Fatalf("expected revoke reason LOGOUT, got %v", revoked.RevokeReason)
	}
}
