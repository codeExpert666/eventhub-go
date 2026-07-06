package response_test

import (
	"context"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/response"
	"eventhub-go/internal/platform/idgen"
)

func TestSuccessMeta(t *testing.T) {
	ctx := idgen.WithRequestID(context.Background(), "req-success")

	meta := response.SuccessMeta(ctx)

	if meta.Code != "COMMON-000" {
		t.Fatalf("unexpected code: %v", meta.Code)
	}
	if meta.Message != "成功" {
		t.Fatalf("unexpected message: %v", meta.Message)
	}
	if meta.RequestID != "req-success" {
		t.Fatalf("unexpected requestId: %v", meta.RequestID)
	}
	if meta.Timestamp.IsZero() {
		t.Fatal("expected timestamp")
	}
}

func TestWriteError(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	request = request.WithContext(idgen.WithRequestID(request.Context(), "req-error"))
	recorder := httptest.NewRecorder()

	response.WriteError(recorder, request, apperror.WithDetails(
		apperror.CommonValidation,
		"请求参数校验失败",
		apperror.Details{"page": "page 必须是整数"},
	))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	body := decodeBody(t, recorder)
	if body["code"] != "COMMON-400" {
		t.Fatalf("unexpected code: %v", body["code"])
	}
	if body["message"] != "请求参数校验失败" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
	if body["requestId"] != "req-error" {
		t.Fatalf("unexpected requestId: %v", body["requestId"])
	}
	data := body["data"].(map[string]any)
	if data["page"] != "page 必须是整数" {
		t.Fatalf("unexpected details: %#v", data)
	}
}

func TestWriteErrorDefaultsNilToInternal(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/test", nil)
	request = request.WithContext(idgen.WithRequestID(request.Context(), "req-nil-error"))
	recorder := httptest.NewRecorder()

	response.WriteError(recorder, request, nil)

	if recorder.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected status: %d", recorder.Code)
	}
	body := decodeBody(t, recorder)
	if body["code"] != "COMMON-500" {
		t.Fatalf("unexpected code: %v", body["code"])
	}
	if body["message"] != "系统内部错误" {
		t.Fatalf("unexpected message: %v", body["message"])
	}
	if body["requestId"] != "req-nil-error" {
		t.Fatalf("unexpected requestId: %v", body["requestId"])
	}
	if body["data"] != nil {
		t.Fatalf("unexpected data: %#v", body["data"])
	}
}

func TestPublicSurfaceStaysFocused(t *testing.T) {
	files := responseFiles(t)

	var functions []string
	var types []string
	for _, file := range files {
		for _, decl := range file.Decls {
			switch decl := decl.(type) {
			case *ast.FuncDecl:
				if decl.Recv == nil && decl.Name.IsExported() {
					functions = append(functions, decl.Name.Name)
				}
			case *ast.GenDecl:
				if decl.Tok != token.TYPE {
					continue
				}
				for _, spec := range decl.Specs {
					typeSpec := spec.(*ast.TypeSpec)
					if typeSpec.Name.IsExported() {
						types = append(types, typeSpec.Name.Name)
					}
				}
			}
		}
	}
	sort.Strings(functions)
	sort.Strings(types)

	wantFunctions := []string{"SuccessMeta", "WriteError"}
	wantTypes := []string{"Meta"}
	if strings.Join(functions, ",") != strings.Join(wantFunctions, ",") {
		t.Fatalf("unexpected exported functions: got %v want %v", functions, wantFunctions)
	}
	if strings.Join(types, ",") != strings.Join(wantTypes, ",") {
		t.Fatalf("unexpected exported types: got %v want %v", types, wantTypes)
	}
}

func TestResponsePackageDoesNotNormalizeErrors(t *testing.T) {
	files := responseFiles(t)
	for _, file := range files {
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if ok && funcDecl.Name.Name == "normalizeError" {
				t.Fatal("response package should use apperror.FromErrorOrInternal instead of a local normalizer")
			}
		}
	}
}

func responseFiles(t *testing.T) []*ast.File {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate test file")
	}
	dir := filepath.Dir(filename)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read response package: %v", err)
	}

	fileSet := token.NewFileSet()
	files := make([]*ast.File, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fileSet, filepath.Join(dir, name), nil, 0)
		if err != nil {
			t.Fatalf("parse response file %s: %v", name, err)
		}
		files = append(files, parsed)
	}
	if len(files) == 0 {
		t.Fatal("response package source files not found")
	}
	return files
}

func decodeBody(t *testing.T, recorder *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	return body
}
