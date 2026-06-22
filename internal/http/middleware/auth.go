package middleware

import (
	"net/http"
	"strings"

	"eventhub-go/internal/apperror"
	"eventhub-go/internal/http/response"
	"eventhub-go/internal/security"
	"eventhub-go/internal/security/jwt"
	usersvc "eventhub-go/internal/service/user"
)

const bearerPrefix = "Bearer "

// Authenticate 返回 Bearer access token 认证 middleware。
func Authenticate(tokens *jwt.Codec, principals *usersvc.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := resolveBearerToken(r)
			if token == "" {
				writeUnauthorized(w, r)
				return
			}
			claims, err := tokens.ParseAccessToken(token)
			if err != nil {
				writeUnauthorized(w, r)
				return
			}
			principal, err := principals.LoadPrincipal(r.Context(), claims.SubjectID)
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
