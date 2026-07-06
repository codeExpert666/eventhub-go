package auth

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"

	authsvc "eventhub-go/internal/service/auth"
	usersvc "eventhub-go/internal/service/user"
)

func TestLogoutStrictClassifiesRequiredPrincipalErrors(t *testing.T) {
	body := requiredPrincipalErrorBranch(t, "LogoutStrict")

	if !callsSelector(body, "errors", "Is") {
		t.Fatal("LogoutStrict should classify security.ErrMissingPrincipal explicitly")
	}
	if !callsSelector(body, "apperror", "FromErrorOrInternal") {
		t.Fatal("LogoutStrict should preserve non-missing principal errors through apperror.FromErrorOrInternal")
	}
}

func TestToOpenAPILoginResponseMapsTokenPairData(t *testing.T) {
	result := authsvc.LoginResult{
		AccessToken:         "access-token",
		RefreshToken:        "refresh-token",
		AuthorizationScheme: "Bearer",
		ExpiresIn:           900,
		RefreshExpiresIn:    7200,
		SessionID:           "session-id",
		User: usersvc.UserResult{
			ID:       42,
			Username: "alice",
			Email:    "alice@example.com",
			Status:   "ENABLED",
		},
	}

	data := toOpenAPILoginResponse(result)

	if data.AccessToken != result.AccessToken ||
		data.RefreshToken != result.RefreshToken ||
		data.AuthorizationScheme != result.AuthorizationScheme ||
		data.ExpiresIn != result.ExpiresIn ||
		data.RefreshExpiresIn != result.RefreshExpiresIn ||
		data.SessionId != result.SessionID ||
		data.User.Id != result.User.ID {
		t.Fatalf("login response data = %#v, want token pair fields from result", data)
	}
	if data.User.Roles == nil || len(data.User.Roles) != 0 {
		t.Fatalf("login response roles = %#v, want empty non-nil roles", data.User.Roles)
	}
}

func TestToOpenAPIRefreshTokenResponseMapsTokenPairData(t *testing.T) {
	result := authsvc.RefreshResult{
		AccessToken:         "new-access-token",
		RefreshToken:        "new-refresh-token",
		AuthorizationScheme: "Bearer",
		ExpiresIn:           300,
		RefreshExpiresIn:    3600,
		SessionID:           "session-id",
		User: usersvc.UserResult{
			ID:       43,
			Username: "bob",
			Email:    "bob@example.com",
			Status:   "ENABLED",
			Roles:    []string{"USER"},
		},
	}

	data := toOpenAPIRefreshTokenResponse(result)

	if data.AccessToken != result.AccessToken ||
		data.RefreshToken != result.RefreshToken ||
		data.AuthorizationScheme != result.AuthorizationScheme ||
		data.ExpiresIn != result.ExpiresIn ||
		data.RefreshExpiresIn != result.RefreshExpiresIn ||
		data.SessionId != result.SessionID ||
		data.User.Id != result.User.ID {
		t.Fatalf("refresh response data = %#v, want token pair fields from result", data)
	}
	if len(data.User.Roles) != 1 || data.User.Roles[0] != "USER" {
		t.Fatalf("refresh response roles = %#v, want USER role", data.User.Roles)
	}
}

func requiredPrincipalErrorBranch(t *testing.T, funcName string) *ast.BlockStmt {
	t.Helper()

	fn := parseStrictFunc(t, funcName)
	for index, stmt := range fn.Body.List {
		if !callsSelector(stmt, "security", "RequiredPrincipal") {
			continue
		}
		if index+1 >= len(fn.Body.List) {
			t.Fatalf("%s should check RequiredPrincipal error", funcName)
		}
		ifStmt, ok := fn.Body.List[index+1].(*ast.IfStmt)
		if !ok {
			t.Fatalf("%s should check RequiredPrincipal error immediately after the call", funcName)
		}
		return ifStmt.Body
	}

	t.Fatalf("%s should call security.RequiredPrincipal", funcName)
	return nil
}

func parseStrictFunc(t *testing.T, funcName string) *ast.FuncDecl {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate auth strict test file")
	}

	fileSet := token.NewFileSet()
	parsed, err := parser.ParseFile(fileSet, filepath.Join(filepath.Dir(filename), "strict.go"), nil, 0)
	if err != nil {
		t.Fatalf("parse auth strict handler: %v", err)
	}
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Name.Name == funcName {
			return fn
		}
	}

	t.Fatalf("function %s not found", funcName)
	return nil
}

func callsSelector(node ast.Node, receiver, name string) bool {
	found := false
	ast.Inspect(node, func(n ast.Node) bool {
		if found || n == nil {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != name {
			return true
		}
		ident, ok := selector.X.(*ast.Ident)
		if !ok || ident.Name != receiver {
			return true
		}
		found = true
		return false
	})
	return found
}
