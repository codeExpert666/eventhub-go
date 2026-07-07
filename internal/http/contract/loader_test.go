package contract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadSpecLoadsResolvesAndValidatesFileSystemSpec(t *testing.T) {
	specPath := writeSpec(t, `openapi: 3.0.3
info:
  title: Contract Loader Test API
  version: test
paths:
  /ping:
    get:
      operationId: ping
      responses:
        "200":
          $ref: "#/components/responses/Pong"
components:
  responses:
    Pong:
      description: pong
`)

	spec, err := LoadSpec(specPath)
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	if spec == nil || spec.Document == nil {
		t.Fatal("expected loaded spec document")
	}
	if spec.Path != specPath {
		t.Fatalf("spec path: got %q want %q", spec.Path, specPath)
	}
	response := spec.Document.Paths.Find("/ping").Get.Responses.Value("200")
	if response == nil || response.Value == nil {
		t.Fatal("expected resolved response ref")
	}
	if response.Value.Description == nil || *response.Value.Description != "pong" {
		t.Fatalf("resolved response description: got %#v", response.Value.Description)
	}
}

func TestLoadSpecFailsForBlankPath(t *testing.T) {
	spec, err := LoadSpec("   ")
	if err == nil {
		t.Fatal("expected blank spec path to fail")
	}
	if spec != nil {
		t.Fatalf("expected nil spec, got %#v", spec)
	}
	if !strings.Contains(err.Error(), "openapi spec path is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadSpecFailsForMissingFile(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing-eventhub.yaml")

	spec, err := LoadSpec(missingPath)
	if err == nil {
		t.Fatal("expected missing spec file to fail")
	}
	if spec != nil {
		t.Fatalf("expected nil spec, got %#v", spec)
	}
	if !strings.Contains(err.Error(), "load openapi spec") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadSpecFailsForInvalidSpec(t *testing.T) {
	specPath := writeSpec(t, `openapi: 3.0.3
info:
  title: Broken Contract Loader Test API
`)

	spec, err := LoadSpec(specPath)
	if err == nil {
		t.Fatal("expected invalid spec to fail")
	}
	if spec != nil {
		t.Fatalf("expected nil spec, got %#v", spec)
	}
	if !strings.Contains(err.Error(), "validate openapi spec") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func writeSpec(t *testing.T, content string) string {
	t.Helper()

	specPath := filepath.Join(t.TempDir(), "eventhub.yaml")
	if err := os.WriteFile(specPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write test spec: %v", err)
	}
	return specPath
}
