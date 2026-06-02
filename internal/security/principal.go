// Package security 定义认证上下文中的最小主体模型。
package security

import (
	"context"
	"errors"
)

// Principal 表示当前请求中已经完成认证的用户主体。
//
// 该对象来自服务端按 JWT sub 回库加载的用户状态和角色，不从 JWT payload 中读取动态用户属性。
type Principal struct {
	UserID      int64
	Username    string
	Authorities []string
}

type principalContextKey struct{}

// ErrMissingPrincipal 表示当前 context 中没有认证主体。
var ErrMissingPrincipal = errors.New("authenticated principal is missing")

// ContextWithPrincipal 将当前认证主体写入 context。
func ContextWithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalContextKey{}, principal)
}

// PrincipalFromContext 从 context 读取当前认证主体。
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalContextKey{}).(Principal)
	return principal, ok && principal.UserID > 0
}

// RequiredPrincipal 从 context 读取当前认证主体；不存在时返回显式错误。
func RequiredPrincipal(ctx context.Context) (Principal, error) {
	principal, ok := PrincipalFromContext(ctx)
	if !ok {
		return Principal{}, ErrMissingPrincipal
	}
	return principal, nil
}
