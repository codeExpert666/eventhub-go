package app

import (
	"context"
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
