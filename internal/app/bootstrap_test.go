package app

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestBootstrapWrapsPlatformProviderErrors(t *testing.T) {
	t.Setenv("EVENTHUB_MYSQL_DSN", "user:pass@tcp(127.0.0.1:3306)/eventhub?timeout=not-a-duration")

	application, err := Bootstrap(context.Background())
	if err == nil {
		t.Fatal("expected bootstrap to fail for invalid mysql dsn")
	}
	if application != nil {
		t.Fatal("expected no application when bootstrap fails")
	}
	if !strings.Contains(err.Error(), "provide platform dependencies") {
		t.Fatalf("expected platform stage in error, got %q", err.Error())
	}
}

func TestBootstrapWrapsOpenAPIAssetProviderErrors(t *testing.T) {
	t.Setenv("EVENTHUB_ENV", "test")
	t.Setenv("EVENTHUB_MYSQL_DSN", "")
	t.Setenv("EVENTHUB_REDIS_ADDR", "")
	t.Setenv("OPENAPI_ENABLED", "true")
	t.Setenv("OPENAPI_ASSET_ROOT", filepath.Join(t.TempDir(), "missing-openapi-assets"))

	application, err := Bootstrap(context.Background())
	if err == nil {
		t.Fatal("expected bootstrap to fail for missing OpenAPI assets")
	}
	if application != nil {
		t.Fatal("expected no application when bootstrap fails")
	}
	if !strings.Contains(err.Error(), "provide http dependencies") {
		t.Fatalf("expected http stage in error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), "openapi asset root") {
		t.Fatalf("expected OpenAPI asset root details, got %q", err.Error())
	}
}
