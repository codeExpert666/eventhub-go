package app

import (
	"net"
	"strings"
	"testing"
)

func TestRunWrapsHTTPServerErrors(t *testing.T) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("listen on random port: %v", err)
	}
	defer listener.Close()

	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split listener address: %v", err)
	}
	t.Setenv("EVENTHUB_HTTP_PORT", port)
	t.Setenv("EVENTHUB_MYSQL_DSN", "")
	t.Setenv("OPENAPI_ENABLED", "false")
	t.Setenv("OPENAPI_REQUEST_VALIDATION_ENABLED", "false")

	err = Run()
	if err == nil {
		t.Fatal("expected run to fail when http port is already in use")
	}
	if !strings.Contains(err.Error(), "run http server") {
		t.Fatalf("expected http server stage in error, got %q", err.Error())
	}
}
