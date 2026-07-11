package user

import (
	"strings"
	"testing"
	"time"

	openapigen "eventhub-go/api/openapi/gen"
	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/requesterror"
	"eventhub-go/internal/page"
)

func TestParseAdminUserListQueryMapsDefaultsNormalizesAndParsesTimes(t *testing.T) {
	username := "  alice  "
	email := "  ALICE@Example.COM  "
	status := openapigen.UserStatus(" ENABLED ")
	createdAtFrom := " 2026-01-01T00:00:00 "
	createdAtTo := "2026-01-02T23:59:59"
	updatedAtFrom := "2026-02-01T01:02:03"
	updatedAtTo := "2026-02-02T04:05:06"

	query, appErr := parseAdminUserListQuery(openapigen.ListAdminUsersParams{
		Username:      &username,
		Email:         &email,
		Status:        &status,
		CreatedAtFrom: &createdAtFrom,
		CreatedAtTo:   &createdAtTo,
		UpdatedAtFrom: &updatedAtFrom,
		UpdatedAtTo:   &updatedAtTo,
	})
	if appErr != nil {
		t.Fatalf("parse admin user list query: %v", appErr)
	}

	if query.Page != page.DefaultPage || query.Size != page.DefaultSize {
		t.Fatalf("pagination = (%d, %d), want defaults (%d, %d)", query.Page, query.Size, page.DefaultPage, page.DefaultSize)
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
	assertTimeParam(t, "createdAtFrom", query.CreatedAtFrom, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
	assertTimeParam(t, "createdAtTo", query.CreatedAtTo, time.Date(2026, 1, 2, 23, 59, 59, 0, time.UTC))
	assertTimeParam(t, "updatedAtFrom", query.UpdatedAtFrom, time.Date(2026, 2, 1, 1, 2, 3, 0, time.UTC))
	assertTimeParam(t, "updatedAtTo", query.UpdatedAtTo, time.Date(2026, 2, 2, 4, 5, 6, 0, time.UTC))
}

func TestParseAdminUserListQueryMapsContractValidatedFieldsWithoutRechecking(t *testing.T) {
	pageNumber := 0
	size := 101
	username := strings.Repeat("u", 33)
	email := strings.Repeat("e", 129)
	status := openapigen.UserStatus("DELETED")
	createdAtFrom := "2026-01-02T00:00:00"
	createdAtTo := "2026-01-01T00:00:00"

	query, appErr := parseAdminUserListQuery(openapigen.ListAdminUsersParams{
		Page:          &pageNumber,
		Size:          &size,
		Username:      &username,
		Email:         &email,
		Status:        &status,
		CreatedAtFrom: &createdAtFrom,
		CreatedAtTo:   &createdAtTo,
	})
	if appErr != nil {
		t.Fatalf("contract-validated fields should be mapped without handler validation: %v", appErr)
	}

	if query.Page != pageNumber || query.Size != size {
		t.Fatalf("pagination = (%d, %d), want (%d, %d)", query.Page, query.Size, pageNumber, size)
	}
	if query.Username != username || query.Email != email || query.Status != string(status) {
		t.Fatalf("filters = (%q, %q, %q), want mapped contract values", query.Username, query.Email, query.Status)
	}
	if query.CreatedAtFrom == nil || query.CreatedAtTo == nil || !query.CreatedAtFrom.After(*query.CreatedAtTo) {
		t.Fatalf("time range = (%v, %v), want reversed values mapped without handler validation", query.CreatedAtFrom, query.CreatedAtTo)
	}
}

func TestParseAdminUserListQueryTreatsTimeParseFailureAsContractInvariant(t *testing.T) {
	invalidCalendarDate := "2026-02-30T00:00:00"

	_, appErr := parseAdminUserListQuery(openapigen.ListAdminUsersParams{CreatedAtFrom: &invalidCalendarDate})
	if appErr == nil {
		t.Fatal("expected contract invariant error")
	}
	if appErr.Code() != apperror.CommonInternal || appErr.Message() != apperror.CommonInternal.DefaultMessage() {
		t.Fatalf("error = (%s, %q), want internal error", appErr.Code(), appErr.Message())
	}
	if appErr.Unwrap() == nil {
		t.Fatal("expected internal error to preserve the time parsing cause")
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

func TestParseUpdateUserStatusCommandMapsContractValidatedFieldsWithoutRechecking(t *testing.T) {
	request := openapigen.UpdateUserStatusRequest{Status: openapigen.UserStatus("DELETED")}

	command, appErr := parseUpdateUserStatusCommand(0, &request)
	if appErr != nil {
		t.Fatalf("contract-validated fields should be mapped without handler validation: %v", appErr)
	}
	if command.UserID != 0 || command.Status != "DELETED" {
		t.Fatalf("command = %#v, want contract values mapped unchanged", command)
	}
}

func TestParseUpdateUserStatusCommandDefendsAgainstNilBodyAsMalformed(t *testing.T) {
	_, appErr := parseUpdateUserStatusCommand(42, nil)
	if appErr == nil {
		t.Fatal("expected malformed body error")
	}
	if appErr.Code() != apperror.CommonValidation || appErr.Message() != "请求体格式不合法" {
		t.Fatalf("error = (%s, %q), want malformed body error", appErr.Code(), appErr.Message())
	}
	violations, ok := appErr.Details()["violations"].(requesterror.Violations)
	if !ok || len(violations) != 1 {
		t.Fatalf("details = %#v, want one violation", appErr.Details())
	}
	want := requesterror.Violation{
		Location: requesterror.LocationBody,
		Field:    "body",
		Path:     "body",
		Rule:     "malformed",
		Message:  "请求体缺失或 JSON 格式错误",
	}
	if violations[0] != want {
		t.Fatalf("violation = %#v, want %#v", violations[0], want)
	}
}

func assertTimeParam(t *testing.T, name string, got *time.Time, want time.Time) {
	t.Helper()
	if got == nil || !got.Equal(want) {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}
