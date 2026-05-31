package requestid_test

import (
	"context"
	"testing"

	"eventhub-go/internal/http/requestid"
)

func TestValid(t *testing.T) {
	validIDs := []string{"abc", "req-123", "REQ_123.456", "1"}
	for _, id := range validIDs {
		if !requestid.Valid(id) {
			t.Fatalf("expected valid request id: %s", id)
		}
	}

	invalidIDs := []string{"", "-starts-with-dash", "has space", "unsafe###", string(make([]byte, 65))}
	for _, id := range invalidIDs {
		if requestid.Valid(id) {
			t.Fatalf("expected invalid request id: %q", id)
		}
	}
}

func TestContextRoundTrip(t *testing.T) {
	ctx := requestid.WithContext(context.Background(), "req-context")
	if got := requestid.FromContext(ctx); got != "req-context" {
		t.Fatalf("unexpected request id: %s", got)
	}
}
