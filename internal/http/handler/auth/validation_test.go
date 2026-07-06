package auth

import (
	"testing"

	openapigen "eventhub-go/api/openapi/gen"

	openapi_types "github.com/oapi-codegen/runtime/types"
)

func TestParseRegisterCommandNormalizesTextFields(t *testing.T) {
	request := openapigen.RegisterRequest{
		Username: "  alice  ",
		Email:    openapi_types.Email("  Alice@Example.COM  "),
		Password: "  Password123  ",
	}
	command, appErr := parseRegisterCommand(&request)
	if appErr != nil {
		t.Fatalf("parse register command: %v", appErr)
	}

	if command.Username != "alice" {
		t.Fatalf("username = %q, want %q", command.Username, "alice")
	}
	if command.Email != "Alice@Example.COM" {
		t.Fatalf("email = %q, want %q", command.Email, "Alice@Example.COM")
	}
	if command.Password != "  Password123  " {
		t.Fatalf("password = %q, want original password", command.Password)
	}
}

func TestParseLoginCommandNormalizesIdentifierOnly(t *testing.T) {
	request := openapigen.LoginRequest{
		UsernameOrEmail: "  Alice@Example.COM  ",
		Password:        "  Password123  ",
	}
	command, appErr := parseLoginCommand(&request)
	if appErr != nil {
		t.Fatalf("parse login command: %v", appErr)
	}

	if command.UsernameOrEmail != "Alice@Example.COM" {
		t.Fatalf("usernameOrEmail = %q, want %q", command.UsernameOrEmail, "Alice@Example.COM")
	}
	if command.Password != "  Password123  " {
		t.Fatalf("password = %q, want original password", command.Password)
	}
}

func TestParseRefreshCommandKeepsTokenOriginal(t *testing.T) {
	request := openapigen.RefreshTokenRequest{
		RefreshToken: "  refresh-token  ",
	}
	command, appErr := parseRefreshCommand(&request)
	if appErr != nil {
		t.Fatalf("parse refresh command: %v", appErr)
	}

	if command.RefreshToken != "  refresh-token  " {
		t.Fatalf("refreshToken = %q, want original token", command.RefreshToken)
	}
}
