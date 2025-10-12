package api

import (
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"kubeop/internal/config"
)

func AdminAuthMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	if cfg.DisableAuth {
		return func(next http.Handler) http.Handler { return next }
	}
	secret := []byte(cfg.AdminJWTSecret)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authz := r.Header.Get("Authorization")
			if authz == "" || !strings.HasPrefix(authz, "Bearer ") {
				http.Error(w, "missing bearer token", http.StatusUnauthorized)
				return
			}
			tokenStr := strings.TrimPrefix(authz, "Bearer ")
			tok, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, jwt.ErrSignatureInvalid
				}
				return secret, nil
			})
			if err != nil || !tok.Valid {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			if claims, ok := tok.Claims.(jwt.MapClaims); ok {
				if role, ok := claims["role"].(string); !ok || role != "admin" {
					http.Error(w, "forbidden", http.StatusForbidden)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
