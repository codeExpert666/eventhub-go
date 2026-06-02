package refresh

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateAndHashRefreshToken(t *testing.T) {
	manager := NewManager(30 * 24 * time.Hour)

	token, err := manager.Generate()
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}
	if len(token) != 43 {
		t.Fatalf("unexpected token length: %d", len(token))
	}
	if _, err := manager.Parse(token); err != nil {
		t.Fatalf("parse generated token: %v", err)
	}
	hash, err := manager.Hash(token)
	if err != nil {
		t.Fatalf("hash token: %v", err)
	}
	if !strings.HasPrefix(hash, "sha256:") || len(hash) != len("sha256:")+64 {
		t.Fatalf("unexpected hash: %s", hash)
	}
	hashAgain, err := manager.Hash(token)
	if err != nil {
		t.Fatalf("hash token again: %v", err)
	}
	if hash != hashAgain {
		t.Fatal("expected stable hash")
	}
}

func TestParseRejectsInvalidRefreshToken(t *testing.T) {
	manager := NewManager(time.Hour)
	for _, token := range []string{"", "abc", strings.Repeat("=", 43), strings.Repeat("a", 44)} {
		if _, err := manager.Parse(token); err == nil {
			t.Fatalf("expected invalid token error for %q", token)
		}
	}
}
