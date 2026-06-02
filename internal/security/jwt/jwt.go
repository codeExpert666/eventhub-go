// Package jwt 提供 JWT access token 签发和解析能力。
package jwt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"eventhub-go/internal/platform/clock"
)

const (
	// AccessTokenType 是 access token 的 typ claim 固定值。
	AccessTokenType = "access"

	algorithmHS256 = "HS256"
	headerTypeJWT  = "JWT"
	minSecretBytes = 32
)

var (
	// ErrInvalidToken 表示 JWT access token 无效。
	ErrInvalidToken = errors.New("invalid jwt access token")
)

// Config 保存 JWT access token 签发和解析配置。
type Config struct {
	Issuer        string
	SigningSecret string
	AccessTTL     time.Duration
	Clock         clock.Clock
}

// Claims 表示 access token 中的最小认证声明。
type Claims struct {
	SubjectID int64
	TokenID   string
	SessionID string
	TokenType string
	Issuer    string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// Codec 负责 access token 的签发、验签、issuer 校验、过期校验和 claim 提取。
type Codec struct {
	issuer        string
	signingSecret []byte
	accessTTL     time.Duration
	clock         clock.Clock
}

type tokenHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

type tokenPayload struct {
	Issuer    string `json:"iss"`
	Subject   string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	TokenID   string `json:"jti"`
	SessionID string `json:"sid"`
	TokenType string `json:"typ"`
}

// NewCodec 创建 JWT codec，并在启动期校验 issuer、密钥和 TTL。
func NewCodec(cfg Config) (*Codec, error) {
	if strings.TrimSpace(cfg.Issuer) == "" {
		return nil, errors.New("jwt issuer is required")
	}
	if len([]byte(cfg.SigningSecret)) < minSecretBytes {
		return nil, errors.New("jwt signing secret must be at least 32 bytes")
	}
	if cfg.AccessTTL <= 0 {
		return nil, errors.New("jwt access token ttl must be positive")
	}
	tokenClock := cfg.Clock
	if tokenClock == nil {
		tokenClock = clock.RealClock{}
	}
	return &Codec{
		issuer:        strings.TrimSpace(cfg.Issuer),
		signingSecret: []byte(cfg.SigningSecret),
		accessTTL:     cfg.AccessTTL,
		clock:         tokenClock,
	}, nil
}

// IssueAccessToken 为指定用户和服务端会话签发 access token。
func (c *Codec) IssueAccessToken(subjectID int64, sessionID string) (string, error) {
	return c.IssueAccessTokenWithTTL(subjectID, sessionID, c.accessTTL)
}

// IssueAccessTokenWithTTL 使用指定 TTL 签发 access token，主要服务测试构造过期 token。
func (c *Codec) IssueAccessTokenWithTTL(subjectID int64, sessionID string, ttl time.Duration) (string, error) {
	if subjectID <= 0 {
		return "", errors.New("jwt subject id must be positive")
	}
	if strings.TrimSpace(sessionID) == "" {
		return "", errors.New("jwt session id is required")
	}
	now := c.clock.Now().UTC()
	payload := tokenPayload{
		Issuer:    c.issuer,
		Subject:   strconv.FormatInt(subjectID, 10),
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
		TokenID:   uuid.NewString(),
		SessionID: strings.TrimSpace(sessionID),
		TokenType: AccessTokenType,
	}
	return c.sign(payload)
}

// ParseAccessToken 校验并解析 access token。
func (c *Codec) ParseAccessToken(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalidToken
	}

	var header tokenHeader
	if err := decodeJSONPart(parts[0], &header); err != nil {
		return Claims{}, fmt.Errorf("%w: malformed header", ErrInvalidToken)
	}
	if header.Algorithm != algorithmHS256 || header.Type != headerTypeJWT {
		return Claims{}, fmt.Errorf("%w: unsupported header", ErrInvalidToken)
	}

	signedPart := parts[0] + "." + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(c.signature(signedPart))) {
		return Claims{}, fmt.Errorf("%w: signature mismatch", ErrInvalidToken)
	}

	var payload tokenPayload
	if err := decodeJSONPart(parts[1], &payload); err != nil {
		return Claims{}, fmt.Errorf("%w: malformed payload", ErrInvalidToken)
	}
	if payload.Issuer != c.issuer {
		return Claims{}, fmt.Errorf("%w: issuer mismatch", ErrInvalidToken)
	}
	if strings.TrimSpace(payload.Subject) == "" {
		return Claims{}, fmt.Errorf("%w: subject is required", ErrInvalidToken)
	}
	subjectID, err := strconv.ParseInt(payload.Subject, 10, 64)
	if err != nil || subjectID <= 0 {
		return Claims{}, fmt.Errorf("%w: subject must be numeric", ErrInvalidToken)
	}
	if strings.TrimSpace(payload.TokenID) == "" {
		return Claims{}, fmt.Errorf("%w: jti is required", ErrInvalidToken)
	}
	if strings.TrimSpace(payload.SessionID) == "" {
		return Claims{}, fmt.Errorf("%w: sid is required", ErrInvalidToken)
	}
	if payload.TokenType != AccessTokenType {
		return Claims{}, fmt.Errorf("%w: typ must be access", ErrInvalidToken)
	}
	expiresAt := time.Unix(payload.ExpiresAt, 0).UTC()
	if !expiresAt.After(c.clock.Now().UTC()) {
		return Claims{}, fmt.Errorf("%w: token expired", ErrInvalidToken)
	}

	return Claims{
		SubjectID: subjectID,
		TokenID:   payload.TokenID,
		SessionID: payload.SessionID,
		TokenType: payload.TokenType,
		Issuer:    payload.Issuer,
		IssuedAt:  time.Unix(payload.IssuedAt, 0).UTC(),
		ExpiresAt: expiresAt,
	}, nil
}

// AccessTokenTTL 返回 access token 有效期。
func (c *Codec) AccessTokenTTL() time.Duration {
	if c == nil {
		return 0
	}
	return c.accessTTL
}

func (c *Codec) sign(payload tokenPayload) (string, error) {
	headerPart, err := encodeJSONPart(tokenHeader{
		Algorithm: algorithmHS256,
		Type:      headerTypeJWT,
	})
	if err != nil {
		return "", err
	}
	payloadPart, err := encodeJSONPart(payload)
	if err != nil {
		return "", err
	}
	signedPart := headerPart + "." + payloadPart
	return signedPart + "." + c.signature(signedPart), nil
}

func (c *Codec) signature(signedPart string) string {
	mac := hmac.New(sha256.New, c.signingSecret)
	_, _ = mac.Write([]byte(signedPart))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func encodeJSONPart(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(encoded), nil
}

func decodeJSONPart(part string, dst any) error {
	decoded, err := base64.RawURLEncoding.DecodeString(part)
	if err != nil {
		return err
	}
	return json.Unmarshal(decoded, dst)
}
