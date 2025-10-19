package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
	"kubeop/internal/config"
	httpmw "kubeop/internal/http/middleware"
	"kubeop/internal/logging"
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
				watcherAuthReject(w, http.StatusUnauthorized, "missing bearer token", nil, nil)
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
				watcherAuthReject(w, http.StatusUnauthorized, "invalid token", nil, err)
				return
			}
			claims, _ := tok.Claims.(jwt.MapClaims)
			if claims == nil {
				watcherAuthReject(w, http.StatusUnauthorized, "invalid token", nil, nil)
				return
			}
			if role, ok := claims["role"].(string); !ok || strings.TrimSpace(role) != "watcher" {
				watcherAuthReject(w, http.StatusForbidden, "forbidden", claims, nil)
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
				if watcherID == "" && clusterID != "" {
					if record, err := svc.GetWatcherByCluster(ctx, clusterID); err == nil {
						watcher = record
						watcherID = strings.TrimSpace(watcher.ID)
						if watcher.ClusterID != "" {
							clusterID = watcher.ClusterID
						}
					} else {
						watcherAuthReject(w, http.StatusUnauthorized, "watcher invalid", claims, err)
						return
					}
				}
				if watcherID == "" {
					watcherAuthReject(w, http.StatusUnauthorized, "watcher id missing", claims, nil)
					return
				}
				lookupCluster := clusterID
				if lookupCluster != "" && lookupCluster == watcherID {
					lookupCluster = ""
				}
				originalWatcherID := watcherID
				var err error
				watcher, err = svc.ValidateWatcher(ctx, watcherID, lookupCluster)
				if err != nil {
					trimmedCluster := strings.TrimSpace(clusterID)
					if trimmedCluster != "" {
						if record, recErr := svc.GetWatcherByCluster(ctx, trimmedCluster); recErr == nil {
							resolvedID := strings.TrimSpace(record.ID)
							resolvedCluster := strings.TrimSpace(record.ClusterID)
							if resolvedID != "" && !record.Disabled {
								if resolvedID != originalWatcherID {
									logging.L().Info("watcher_auth_resolved_by_cluster",
										zap.String("cluster_id", resolvedCluster),
										zap.String("original_watcher_id", originalWatcherID),
										zap.String("resolved_watcher_id", resolvedID),
									)
								}
								watcher = record
								watcherID = resolvedID
								if resolvedCluster != "" {
									clusterID = resolvedCluster
								}
								err = nil
							}
						}
					}
					if err != nil {
						watcherAuthReject(w, http.StatusUnauthorized, "watcher invalid", claims, err)
						return
					}
				}
				watcherID = strings.TrimSpace(watcher.ID)
				if strings.TrimSpace(watcher.ClusterID) != "" {
					clusterID = strings.TrimSpace(watcher.ClusterID)
				}
			} else if watcherID == "" {
				watcherAuthReject(w, http.StatusUnauthorized, "watcher id missing", claims, nil)
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

func watcherAuthReject(w http.ResponseWriter, status int, message string, claims jwt.MapClaims, cause error) {
	fields := []zap.Field{
		zap.Int("status", status),
		zap.String("message", message),
	}
	if summary := summarizeWatcherClaims(claims); len(summary) > 0 {
		fields = append(fields, zap.Any("claims", summary))
	}
	if cause != nil {
		fields = append(fields, zap.Error(cause))
	}
	logging.L().Warn("watcher_auth_reject", fields...)
	http.Error(w, message, status)
}

func summarizeWatcherClaims(claims jwt.MapClaims) map[string]any {
	if claims == nil {
		return nil
	}
	summary := map[string]any{}
	if watcherID, ok := claims["watcher_id"].(string); ok {
		watcherID = strings.TrimSpace(watcherID)
		if watcherID != "" {
			summary["watcherId"] = watcherID
		}
	}
	if clusterID, ok := claims["cluster_id"].(string); ok {
		clusterID = strings.TrimSpace(clusterID)
		if clusterID != "" {
			summary["clusterId"] = clusterID
		}
	}
	if sub, ok := claims["sub"].(string); ok {
		sub = strings.TrimSpace(sub)
		if sub != "" {
			summary["subject"] = sub
		}
	}
	if iss, ok := claims["iss"].(string); ok {
		iss = strings.TrimSpace(iss)
		if iss != "" {
			summary["issuer"] = iss
		}
	}
	if aud := extractAudience(claims["aud"]); len(aud) > 0 {
		summary["audience"] = aud
	}
	if expTime, ok := extractExpiry(claims["exp"]); ok {
		summary["expiresAt"] = expTime.UTC().Format(time.RFC3339)
	}
	return summary
}

func extractAudience(raw interface{}) []string {
	switch v := raw.(type) {
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil
		}
		return []string{trimmed}
	case []interface{}:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				s = strings.TrimSpace(s)
				if s != "" {
					result = append(result, s)
				}
			}
		}
		return result
	case []string:
		result := make([]string, 0, len(v))
		for _, s := range v {
			s = strings.TrimSpace(s)
			if s != "" {
				result = append(result, s)
			}
		}
		return result
	default:
		return nil
	}
}

func extractExpiry(raw interface{}) (time.Time, bool) {
	switch v := raw.(type) {
	case nil:
		return time.Time{}, false
	case *jwt.NumericDate:
		if v == nil {
			return time.Time{}, false
		}
		return v.Time, true
	case jwt.NumericDate:
		return v.Time, true
	case float64:
		return time.Unix(int64(v), 0), true
	case int64:
		return time.Unix(v, 0), true
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return time.Unix(i, 0), true
		}
	case string:
		if trimmed := strings.TrimSpace(v); trimmed != "" {
			if i, err := json.Number(trimmed).Int64(); err == nil {
				return time.Unix(i, 0), true
			}
		}
	}
	return time.Time{}, false
}
