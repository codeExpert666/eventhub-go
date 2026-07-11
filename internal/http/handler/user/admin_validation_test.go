package user

import (
	"testing"

	openapigen "eventhub-go/api/openapi/gen"
	"eventhub-go/internal/http/requesterror"
)

func TestParseAdminUserListQueryNormalizesTextFilters(t *testing.T) {
	username := "  alice  "
	email := "  ALICE@Example.COM  "
	status := openapigen.UserStatus(" ENABLED ")

	query, appErr := parseAdminUserListQuery(openapigen.ListAdminUsersParams{
		Username: &username,
		Email:    &email,
		Status:   &status,
	})
	if appErr != nil {
		t.Fatalf("parse admin user list query: %v", appErr)
	}

	if query.Username != "alice" {
		t.Fatalf("username = %q, want %q", query.Username, "alice")
	}
	if query.Email != "ALICE@Example.COM" {
		t.Fatalf("email = %q, want %q", query.Email, "ALICE@Example.COM")
	}
	if query.Status != "ENABLED" {
		t.Fatalf("status = %q, want %q", query.Status, "ENABLED")
	}
}

func TestParseUpdateUserStatusCommandCombinesPathAndBody(t *testing.T) {
	request := openapigen.UpdateUserStatusRequest{
		Status: openapigen.DISABLED,
	}
	command, appErr := parseUpdateUserStatusCommand(42, &request)
	if appErr != nil {
		t.Fatalf("parse update user status command: %v", appErr)
	}

	if command.UserID != 42 {
		t.Fatalf("userID = %d, want 42", command.UserID)
	}
	if command.Status != "DISABLED" {
		t.Fatalf("status = %q, want %q", command.Status, "DISABLED")
	}
}

func TestParseUpdateUserStatusCommandValidatesPathBeforeBody(t *testing.T) {
	_, appErr := parseUpdateUserStatusCommand(0, nil)
	if appErr == nil {
		t.Fatal("expected path validation error")
	}
	violations, ok := appErr.Details()["violations"].(requesterror.Violations)
	if !ok || len(violations) != 1 {
		t.Fatalf("details = %#v, want one violation", appErr.Details())
	}
	want := requesterror.Violation{
		Location: requesterror.LocationPath,
		Field:    "userId",
		Path:     "userId",
		Rule:     "minimum",
		Message:  "userId 必须是正整数",
	}
	if violations[0] != want {
		t.Fatalf("violation = %#v, want %#v", violations[0], want)
	}
}
