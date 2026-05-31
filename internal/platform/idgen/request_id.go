// Package idgen provides identifiers used across platform boundaries.
package idgen

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"
)

// HeaderRequestID is the HTTP header used to carry request ids.
const HeaderRequestID = "X-Request-Id"

type requestIDContextKey struct{}

// WithRequestID returns a context carrying requestID.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

// RequestIDFromContext returns the request id stored in ctx, if present.
func RequestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}

// NewRequestID generates a new request id.
func NewRequestID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}
	return hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405.000000000")))
}

// ValidRequestID reports whether id is safe to echo in headers, logs, and JSON responses.
func ValidRequestID(id string) bool {
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
