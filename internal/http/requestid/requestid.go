package requestid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"
)

const HeaderName = "X-Request-Id"

type contextKey struct{}

func WithContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, contextKey{}, id)
}

func FromContext(ctx context.Context) string {
	id, _ := ctx.Value(contextKey{}).(string)
	return id
}

func New() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}
	return hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405.000000000")))
}

func Valid(id string) bool {
	if len(id) == 0 || len(id) > 64 {
		return false
	}
	if !isAlphaNumeric(id[0]) {
		return false
	}
	for i := 1; i < len(id); i++ {
		if !isAllowed(id[i]) {
			return false
		}
	}
	return true
}

func isAllowed(ch byte) bool {
	return isAlphaNumeric(ch) || ch == '.' || ch == '_' || ch == '-'
}

func isAlphaNumeric(ch byte) bool {
	return (ch >= 'A' && ch <= 'Z') ||
		(ch >= 'a' && ch <= 'z') ||
		(ch >= '0' && ch <= '9')
}
