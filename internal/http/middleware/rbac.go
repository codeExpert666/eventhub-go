package middleware

import (
	"net/http"
	"strings"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/response"
	"eventhub-go/internal/security"
)

const roleAuthorityPrefix = "ROLE_"

// RequireRole 要求当前认证主体拥有指定业务角色。
func RequireRole(role string) func(http.Handler) http.Handler {
	requiredAuthority := normalizeAuthority(role)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			principal, ok := security.PrincipalFromContext(r.Context())
			if !ok {
				response.WriteError(w, r, apperror.New(apperror.AuthUnauthorized, "请先登录或重新登录"))
				return
			}
			if !hasAuthority(principal.Authorities, requiredAuthority) {
				response.WriteError(w, r, apperror.New(apperror.AuthForbidden, "权限不足"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func normalizeAuthority(role string) string {
	normalized := strings.ToUpper(strings.TrimSpace(role))
	if strings.HasPrefix(normalized, roleAuthorityPrefix) {
		return normalized
	}
	return roleAuthorityPrefix + normalized
}

func hasAuthority(authorities []string, required string) bool {
	for _, authority := range authorities {
		if strings.ToUpper(strings.TrimSpace(authority)) == required {
			return true
		}
	}
	return false
}
