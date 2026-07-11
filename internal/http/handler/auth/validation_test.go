package auth

import (
	"reflect"
	"strings"
	"testing"

	openapigen "eventhub-go/api/openapi/gen"
	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/requesterror"

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

func TestAuthParsersMapContractInvalidFieldsWithoutHandlerErrors(t *testing.T) {
	t.Run("register", func(t *testing.T) {
		request := openapigen.RegisterRequest{
			Username: "  !  ",
			Email:    openapi_types.Email("not-an-email"),
			Password: "short",
		}

		command, appErr := parseRegisterCommand(&request)
		if appErr != nil {
			t.Fatalf("parse contract-invalid register request: %v", appErr)
		}
		if command.Username != "!" || command.Email != "not-an-email" || command.Password != "short" {
			t.Fatalf("register command = %#v, want normalized field mapping only", command)
		}
	})

	t.Run("register maximum lengths", func(t *testing.T) {
		request := openapigen.RegisterRequest{
			Username: strings.Repeat("u", 33),
			Email:    openapi_types.Email(strings.Repeat("e", 129)),
			Password: strings.Repeat("p", 73),
		}

		command, appErr := parseRegisterCommand(&request)
		if appErr != nil {
			t.Fatalf("parse contract-invalid register lengths: %v", appErr)
		}
		if command.Username != request.Username || command.Email != string(request.Email) || command.Password != request.Password {
			t.Fatalf("register command = %#v, want overlong fields mapped without validation", command)
		}
	})

	t.Run("login", func(t *testing.T) {
		request := openapigen.LoginRequest{
			UsernameOrEmail: strings.Repeat("i", 129),
			Password:        strings.Repeat("p", 73),
		}

		command, appErr := parseLoginCommand(&request)
		if appErr != nil {
			t.Fatalf("parse contract-invalid login request: %v", appErr)
		}
		if command.UsernameOrEmail != request.UsernameOrEmail || command.Password != request.Password {
			t.Fatalf("login command = %#v, want normalized field mapping only", command)
		}
	})

	t.Run("refresh", func(t *testing.T) {
		request := openapigen.RefreshTokenRequest{RefreshToken: strings.Repeat("r", 129)}

		command, appErr := parseRefreshCommand(&request)
		if appErr != nil {
			t.Fatalf("parse contract-invalid refresh request: %v", appErr)
		}
		if command.RefreshToken != request.RefreshToken {
			t.Fatalf("refresh command = %#v, want original token mapping", command)
		}
	})
}

func TestAuthParsersClassifyNilBodyAsMalformed(t *testing.T) {
	tests := []struct {
		name  string
		parse func() *apperror.AppError
	}{
		{
			name: "register",
			parse: func() *apperror.AppError {
				_, appErr := parseRegisterCommand(nil)
				return appErr
			},
		},
		{
			name: "login",
			parse: func() *apperror.AppError {
				_, appErr := parseLoginCommand(nil)
				return appErr
			},
		},
		{
			name: "refresh",
			parse: func() *apperror.AppError {
				_, appErr := parseRefreshCommand(nil)
				return appErr
			},
		},
	}
	want := requesterror.MalformedBody()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.parse()
			if got == nil {
				t.Fatal("nil body error = nil, want malformed body")
			}
			if got.Code() != want.Code() || got.Message() != want.Message() || !reflect.DeepEqual(got.Details(), want.Details()) {
				t.Fatalf("nil body error = %#v, want %#v", got, want)
			}
		})
	}
}
