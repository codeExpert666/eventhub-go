// Package idgen 提供跨平台基础设施边界使用的标识生成能力。
package idgen

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"
)

// HeaderRequestID 是用于传递请求 ID 的 HTTP 请求头名称。
const HeaderRequestID = "X-Request-Id"

// requestIDContextKey 是 request ID 在 context 中的私有键类型。
//
// 使用私有空结构体可以避免与其他 package 写入 context 的键发生冲突。
type requestIDContextKey struct{}

// WithRequestID 返回一个携带 requestID 的派生 context。
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDContextKey{}, requestID)
}

// RequestIDFromContext 返回 ctx 中存储的请求 ID。
//
// 当 ctx 中不存在请求 ID 或值类型不是 string 时，返回空字符串。
func RequestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDContextKey{}).(string)
	return requestID
}

// NewRequestID 生成新的请求 ID。
//
// 正常情况下返回 16 字节随机数的十六进制字符串；当系统随机源不可用时，
// 使用当前 UTC 时间戳作为降级来源，确保仍能生成可用于追踪的请求 ID。
func NewRequestID() string {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}
	return hex.EncodeToString([]byte(time.Now().UTC().Format("20060102150405.000000000")))
}

// ValidRequestID 判断 id 是否可以安全地回显到响应头、日志和 JSON 响应中。
//
// 合法请求 ID 必须非空、长度不超过 64 字节、首字符为字母或数字，
// 其余字符只允许字母、数字、点号、下划线和连字符。
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

// isAllowed 判断字符是否属于请求 ID 允许使用的字符集。
func isAllowed(ch byte) bool {
	return isAlphaNumeric(ch) || ch == '.' || ch == '_' || ch == '-'
}

// isAlphaNumeric 判断字符是否为 ASCII 字母或数字。
func isAlphaNumeric(ch byte) bool {
	return (ch >= 'A' && ch <= 'Z') ||
		(ch >= 'a' && ch <= 'z') ||
		(ch >= '0' && ch <= '9')
}
