package http_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const (
	appErrorImportPath     = "eventhub-go/internal/apperror"
	requestErrorImportPath = "eventhub-go/internal/http/requesterror"
)

func TestBusinessHandlersLeaveFieldValidationToOpenAPIContract(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate handler validation boundary test")
	}

	for _, module := range []string{"auth", "user", "system"} {
		t.Run(module, func(t *testing.T) {
			directory := filepath.Join(filepath.Dir(testFile), "handler", module)
			files, err := filepath.Glob(filepath.Join(directory, "*.go"))
			if err != nil {
				t.Fatalf("list %s handler files: %v", module, err)
			}
			for _, filename := range files {
				if filepath.Ext(filename) != ".go" || strings.HasSuffix(filename, "_test.go") {
					continue
				}
				assertHandlerValidationBoundary(t, filename)
			}
		})
	}
}

func assertHandlerValidationBoundary(t *testing.T, filename string) {
	t.Helper()

	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, filename, nil, 0)
	if err != nil {
		t.Fatalf("parse handler %s: %v", filename, err)
	}

	appErrorAliases := map[string]struct{}{}
	requestErrorAliases := map[string]struct{}{}
	for _, declaration := range file.Imports {
		importPath, err := strconv.Unquote(declaration.Path.Value)
		if err != nil {
			t.Fatalf("decode import in %s: %v", filename, err)
		}
		if importPath == "net/mail" || importPath == "regexp" {
			t.Errorf("%s imports %s; field validation belongs to internal/http/contract", filename, importPath)
		}
		switch importPath {
		case appErrorImportPath:
			alias, ok := importAlias(declaration, "apperror")
			if !ok {
				t.Errorf("%s must not dot/blank import apperror", filename)
				continue
			}
			appErrorAliases[alias] = struct{}{}
		case requestErrorImportPath:
			alias, ok := importAlias(declaration, "requesterror")
			if !ok {
				t.Errorf("%s must not dot/blank import requesterror", filename)
				continue
			}
			requestErrorAliases[alias] = struct{}{}
		}
	}

	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		identifier, ok := selector.X.(*ast.Ident)
		if !ok {
			return true
		}
		if _, ok := requestErrorAliases[identifier.Name]; ok {
			if selector.Sel.Name != "MalformedBody" {
				position := fileSet.Position(selector.Pos())
				t.Errorf(
					"%s:%d uses requesterror.%s; handlers may only defend nil bodies with MalformedBody",
					filename,
					position.Line,
					selector.Sel.Name,
				)
			}
		}
		if _, ok := appErrorAliases[identifier.Name]; ok &&
			(selector.Sel.Name == "CommonValidation" || selector.Sel.Name == "WithDetails") {
			position := fileSet.Position(selector.Pos())
			t.Errorf(
				"%s:%d uses apperror.%s; field validation belongs to internal/http/contract",
				filename,
				position.Line,
				selector.Sel.Name,
			)
		}
		return true
	})
}

func importAlias(declaration *ast.ImportSpec, defaultAlias string) (string, bool) {
	if declaration.Name == nil {
		return defaultAlias, true
	}
	alias := declaration.Name.Name
	return alias, alias != "." && alias != "_"
}
