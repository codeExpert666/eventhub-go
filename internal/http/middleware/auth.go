package middleware

import (
	"context"
	"net/http"
	"strings"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/response"
	"eventhub-go/internal/security"
	"eventhub-go/internal/security/jwt"
)

const bearerPrefix = "Bearer "

// AccessTokenParser 表示 access token 解析能力。
type AccessTokenParser interface {
	ParseAccessToken(token string) (jwt.Claims, error)
}

// PrincipalLoader 表示按用户 ID 加载认证主体的能力。
type PrincipalLoader interface {
	LoadPrincipal(ctx context.Context, userID int64) (security.Principal, error)
}

// AuthMiddleware 负责 Bearer access token 认证。
type AuthMiddleware struct {
	tokens     AccessTokenParser
	principals PrincipalLoader
}

// NewAuth 创建 Bearer access token 认证 middleware。
func NewAuth(tokens AccessTokenParser, principals PrincipalLoader) *AuthMiddleware {
	return &AuthMiddleware{tokens: tokens, principals: principals}
}

// Middleware 校验 Bearer token，并把已认证主体写入请求 context。
func (m *AuthMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := resolveBearerToken(r)
		if token == "" {
			writeUnauthorized(w, r)
			return
		}
		claims, err := m.tokens.ParseAccessToken(token)
		if err != nil {
			writeUnauthorized(w, r)
			return
		}
		principal, err := m.principals.LoadPrincipal(r.Context(), claims.SubjectID)
		if err != nil {
			if appErr, ok := apperror.FromError(err); ok && appErr.Code() == apperror.AuthUnauthorized {
				writeUnauthorized(w, r)
				return
			}
			response.WriteError(w, r, apperror.Wrap(apperror.CommonInternal, "", err))
			return
		}
		next.ServeHTTP(w, r.WithContext(security.ContextWithPrincipal(r.Context(), principal)))
	})
}

func resolveBearerToken(r *http.Request) string {
	authorization := r.Header.Get("Authorization")
	if !strings.HasPrefix(authorization, bearerPrefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(authorization, bearerPrefix))
}

func writeUnauthorized(w http.ResponseWriter, r *http.Request) {
	response.WriteError(w, r, apperror.New(apperror.AuthUnauthorized, "请先登录或重新登录"))
}
