// Package middleware 提供 HTTP 中间件。
package middleware

import (
	"context"
	"net/http"
	"strings"

	"shadow-worker/update_service/internal/service"
)

type contextKey string

const contextUsername contextKey = "username"

// Auth 校验 JWT，并把用户名注入请求上下文。
func Auth(authSvc *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" {
				http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
				return
			}
			parts := strings.SplitN(header, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, `{"error":"invalid authorization header"}`, http.StatusUnauthorized)
				return
			}
			username, err := authSvc.ValidateToken(parts[1])
			if err != nil {
				http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
				return
			}
			ctx := context.WithValue(r.Context(), contextUsername, username)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UsernameFromContext 从上下文取用户名。
func UsernameFromContext(ctx context.Context) string {
	v, _ := ctx.Value(contextUsername).(string)
	return v
}
