package idgen_test

import (
	"context"
	"testing"

	"eventhub-go/internal/platform/idgen"
)

func TestValidRequestID(t *testing.T) {
	validIDs := []string{"abc", "req-123", "REQ_123.456", "1"}
	for _, id := range validIDs {
		if !idgen.ValidRequestID(id) {
			t.Fatalf("expected valid request id: %s", id)
		}
	}

	invalidIDs := []string{"", "-starts-with-dash", "has space", "unsafe###", string(make([]byte, 65))}
	for _, id := range invalidIDs {
		if idgen.ValidRequestID(id) {
			t.Fatalf("expected invalid request id: %q", id)
		}
	}
}

func TestRequestIDContextRoundTrip(t *testing.T) {
	ctx := idgen.WithRequestID(context.Background(), "req-context")
	if got := idgen.RequestIDFromContext(ctx); got != "req-context" {
		t.Fatalf("unexpected request id: %s", got)
	}
}
