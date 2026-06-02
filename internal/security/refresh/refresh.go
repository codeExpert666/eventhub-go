// Package refresh 提供 opaque refresh token 生成、校验和哈希能力。
package refresh

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"regexp"
	"time"
)

const (
	randomByteLength = 32
	plainTokenLength = 43
	storedPrefix     = "sha256:"
)

var urlSafeBase64WithoutPadding = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// ErrInvalidToken 表示 refresh token 明文格式不合法。
var ErrInvalidToken = errors.New("invalid refresh token")

// Manager 管理 opaque refresh token 的生命周期参数和哈希格式。
type Manager struct {
	ttl    time.Duration
	reader io.Reader
}

// NewManager 创建 refresh token 管理器。
func NewManager(ttl time.Duration) *Manager {
	return &Manager{ttl: ttl, reader: rand.Reader}
}

// Generate 生成 32 字节随机 opaque refresh token，并使用 Base64 URL-safe 无 padding 编码。
func (m *Manager) Generate() (string, error) {
	reader := rand.Reader
	if m != nil && m.reader != nil {
		reader = m.reader
	}
	randomBytes := make([]byte, randomByteLength)
	if _, err := io.ReadFull(reader, randomBytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(randomBytes), nil
}

// Parse 校验 refresh token 明文格式，成功时返回原始 token。
func (m *Manager) Parse(token string) (string, error) {
	if len(token) != plainTokenLength || !urlSafeBase64WithoutPadding.MatchString(token) {
		return "", ErrInvalidToken
	}
	return token, nil
}

// Hash 生成带算法前缀的 refresh token hash。
func (m *Manager) Hash(token string) (string, error) {
	if _, err := m.Parse(token); err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(token))
	return storedPrefix + hex.EncodeToString(sum[:]), nil
}

// RefreshTokenTTL 返回 refresh token 有效期。
func (m *Manager) RefreshTokenTTL() time.Duration {
	if m == nil || m.ttl <= 0 {
		return 30 * 24 * time.Hour
	}
	return m.ttl
}
