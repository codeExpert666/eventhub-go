package jwt

import (
	"strings"
	"testing"
	"time"
)

const testSecret = "eventhub-test-access-token-secret-for-jwt-codec"

func TestIssueAndParseAccessTokenKeepsRequiredClaims(t *testing.T) {
	codec := newCodecForTest(t)

	token, err := codec.IssueAccessToken(1001, "session-1001", 30*time.Minute)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	claims, err := codec.ParseAccessToken(token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}

	if claims.SubjectID != 1001 || claims.SessionID != "session-1001" || claims.TokenType != AccessTokenType {
		t.Fatalf("unexpected claims: %#v", claims)
	}
	if claims.TokenID == "" || claims.Issuer != "eventhub-backend" {
		t.Fatalf("missing required claims: %#v", claims)
	}
}

func TestParseAccessTokenRejectsWrongTypeAndMissingRequiredClaims(t *testing.T) {
	codec := newCodecForTest(t)

	tests := []struct {
		name   string
		mutate func(tokenPayload) tokenPayload
	}{
		{
			name: "wrong typ",
			mutate: func(payload tokenPayload) tokenPayload {
				payload.TokenType = "refresh"
				return payload
			},
		},
		{
			name: "missing jti",
			mutate: func(payload tokenPayload) tokenPayload {
				payload.TokenID = ""
				return payload
			},
		},
		{
			name: "missing sid",
			mutate: func(payload tokenPayload) tokenPayload {
				payload.SessionID = ""
				return payload
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			token := signPayloadForTest(t, codec, tt.mutate(validPayloadForTest()))
			if _, err := codec.ParseAccessToken(token); err == nil {
				t.Fatal("expected parse error")
			}
		})
	}
}

func TestParseAccessTokenRejectsExpiredAndTamperedToken(t *testing.T) {
	codec := newCodecForTest(t)
	expired, err := codec.IssueAccessToken(1001, "session-1001", -time.Second)
	if err != nil {
		t.Fatalf("issue expired token: %v", err)
	}
	if _, err := codec.ParseAccessToken(expired); err == nil {
		t.Fatal("expected expired token error")
	}

	valid, err := codec.IssueAccessToken(1001, "session-1001", 30*time.Minute)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	tampered := valid[:len(valid)-1] + "x"
	if strings.HasSuffix(valid, "x") {
		tampered = valid[:len(valid)-1] + "y"
	}
	if _, err := codec.ParseAccessToken(tampered); err == nil {
		t.Fatal("expected tampered token error")
	}
}

func newCodecForTest(t *testing.T) *Codec {
	t.Helper()
	codec, err := NewCodec("eventhub-backend", testSecret, nil)
	if err != nil {
		t.Fatalf("new codec: %v", err)
	}
	return codec
}

func validPayloadForTest() tokenPayload {
	now := time.Now().UTC()
	return tokenPayload{
		Issuer:    "eventhub-backend",
		Subject:   "1001",
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(30 * time.Minute).Unix(),
		TokenID:   "jwt-id-1001",
		SessionID: "session-1001",
		TokenType: AccessTokenType,
	}
}

func signPayloadForTest(t *testing.T, codec *Codec, payload tokenPayload) string {
	t.Helper()
	token, err := codec.sign(payload)
	if err != nil {
		t.Fatalf("sign payload: %v", err)
	}
	return token
}
