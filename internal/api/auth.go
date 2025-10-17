package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"kubeop/internal/config"
	httpmw "kubeop/internal/http/middleware"
	"kubeop/internal/service"
	"kubeop/internal/store"
)

type ctxClaimsKey struct{}

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
			claims, _ := tok.Claims.(jwt.MapClaims)
			if claims == nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			actor := ""
			if role, ok := claims["role"].(string); !ok || role != "admin" {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			if sub, ok := claims["sub"].(string); ok && strings.TrimSpace(sub) != "" {
				actor = strings.TrimSpace(sub)
			}
			if actor == "" {
				if uid, ok := claims["user_id"].(string); ok {
					actor = strings.TrimSpace(uid)
				}
			}
			if actor == "" {
				if email, ok := claims["email"].(string); ok {
					actor = strings.TrimSpace(email)
				}
			}
			r = r.Clone(context.WithValue(r.Context(), ctxClaimsKey{}, claims))
			if actor != "" {
				r = httpmw.WithUserID(r, actor)
			}
			next.ServeHTTP(w, r)
		})
	}
}

func claimsFromContext(ctx context.Context) jwt.MapClaims {
	if ctx == nil {
		return nil
	}
	if claims, ok := ctx.Value(ctxClaimsKey{}).(jwt.MapClaims); ok {
		return claims
	}
	return nil
}

func clusterIDFromClaims(claims jwt.MapClaims) string {
	if claims == nil {
		return ""
	}
	if cid, ok := claims["cluster_id"].(string); ok && strings.TrimSpace(cid) != "" {
		return strings.TrimSpace(cid)
	}
	if sub, ok := claims["sub"].(string); ok {
		parts := strings.SplitN(sub, ":", 2)
		if len(parts) == 2 {
			if strings.TrimSpace(parts[0]) == "watcher" {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

func watcherIDFromClaims(claims jwt.MapClaims) string {
	if claims == nil {
		return ""
	}
	if wid, ok := claims["watcher_id"].(string); ok && strings.TrimSpace(wid) != "" {
		return strings.TrimSpace(wid)
	}
	if sub, ok := claims["sub"].(string); ok {
		parts := strings.SplitN(sub, ":", 2)
		if len(parts) == 2 && strings.TrimSpace(parts[0]) == "watcher" {
			return strings.TrimSpace(parts[1])
		}
	}
	return ""
}

func WatcherAuthMiddleware(cfg *config.Config, svc *service.Service) func(http.Handler) http.Handler {
	if cfg != nil && cfg.DisableAuth {
		return func(next http.Handler) http.Handler { return next }
	}
	secret := ""
	if cfg != nil {
		secret = cfg.AdminJWTSecret
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if secret == "" {
				http.Error(w, "watcher auth misconfigured", http.StatusInternalServerError)
				return
			}
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
				return []byte(secret), nil
			})
			if err != nil || !tok.Valid {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			claims, _ := tok.Claims.(jwt.MapClaims)
			if claims == nil {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			if role, ok := claims["role"].(string); !ok || strings.TrimSpace(role) != "watcher" {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			watcherID := ""
			if id, ok := claims["watcher_id"].(string); ok {
				watcherID = strings.TrimSpace(id)
			}
			if watcherID == "" {
				if sub, ok := claims["sub"].(string); ok {
					parts := strings.SplitN(sub, ":", 2)
					if len(parts) == 2 && strings.TrimSpace(parts[0]) == "watcher" {
						watcherID = strings.TrimSpace(parts[1])
					}
				}
			}
			clusterID := clusterIDFromClaims(claims)
			var watcher store.Watcher
			if svc != nil {
				ctx := r.Context()
				resolved := false
				if watcherID != "" {
					lookupCluster := clusterID
					if lookupCluster != "" && lookupCluster == watcherID {
						lookupCluster = ""
					}
					if record, err := svc.ValidateWatcher(ctx, watcherID, lookupCluster); err == nil {
						watcher = record
						resolved = true
					} else if clusterID != "" {
						if record, err := svc.GetWatcherByCluster(ctx, clusterID); err == nil {
							watcher = record
							watcherID = strings.TrimSpace(watcher.ID)
							resolved = true
						} else {
							http.Error(w, "watcher invalid", http.StatusUnauthorized)
							return
						}
					} else {
						http.Error(w, "watcher invalid", http.StatusUnauthorized)
						return
					}
				} else if clusterID != "" {
					if record, err := svc.GetWatcherByCluster(ctx, clusterID); err == nil {
						watcher = record
						watcherID = strings.TrimSpace(watcher.ID)
						resolved = true
					} else {
						http.Error(w, "watcher invalid", http.StatusUnauthorized)
						return
					}
				} else {
					http.Error(w, "watcher id missing", http.StatusUnauthorized)
					return
				}
				if !resolved {
					http.Error(w, "watcher invalid", http.StatusUnauthorized)
					return
				}
				watcherID = strings.TrimSpace(watcher.ID)
				if watcherID == "" {
					http.Error(w, "watcher id missing", http.StatusUnauthorized)
					return
				}
				if strings.TrimSpace(watcher.ClusterID) != "" {
					clusterID = strings.TrimSpace(watcher.ClusterID)
				}
			} else if watcherID == "" {
				http.Error(w, "watcher id missing", http.StatusUnauthorized)
				return
			}
			if claims == nil {
				claims = jwt.MapClaims{}
			}
			if watcherID != "" {
				claims["watcher_id"] = watcherID
			}
			if clusterID != "" {
				claims["cluster_id"] = clusterID
			}
			r = r.Clone(context.WithValue(r.Context(), ctxClaimsKey{}, claims))
			if watcherID != "" {
				r = httpmw.WithUserID(r, watcherID)
			}
			next.ServeHTTP(w, r)
		})
	}
}
