package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"
	"kubeop/internal/logging"
)

// AuditLog records mutating requests when audit logging is enabled.
func AuditLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isAuditMethod(r.Method) || shouldSkipAudit(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		ww := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(ww, r)

		reason := ""
		if ww.status >= http.StatusBadRequest {
			reason = redactError(ww.errMsg)
			if reason == "" {
				reason = http.StatusText(ww.status)
			}
		}

		fields := []zap.Field{
			zap.String("when", time.Now().UTC().Format(time.RFC3339Nano)),
			zap.String("request_id", middleware.GetReqID(r.Context())),
			zap.String("actor_user_id", userID(r)),
			zap.String("tenant_id", tenantID(r)),
			zap.String("verb", strings.ToUpper(r.Method)),
			zap.String("resource", auditResource(r.URL.Path)),
			zap.String("resource_id", auditResourceID(r.URL.Path)),
			zap.Int("status", ww.status),
			zap.String("ip", remoteIP(r)),
		}
		if reason != "" {
			fields = append(fields, zap.String("reason", reason))
		}
		logging.Audit().Info("audit", fields...)
	})
}

func isAuditMethod(method string) bool {
	switch strings.ToUpper(method) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func shouldSkipAudit(path string) bool {
	if path == "" {
		return false
	}
	if isProbe(path) {
		return true
	}
	if strings.HasPrefix(path, "/metrics") {
		return true
	}
	return false
}

func auditResource(path string) string {
	segments := splitPath(path)
	if len(segments) == 0 {
		return ""
	}
	if segments[0] == "v1" {
		if len(segments) > 1 {
			return segments[1]
		}
		return "v1"
	}
	return segments[0]
}

func auditResourceID(path string) string {
	segments := splitPath(path)
	if len(segments) == 0 {
		return ""
	}
	if segments[0] == "v1" && len(segments) > 2 {
		return strings.Join(segments[2:], "/")
	}
	if len(segments) > 1 {
		return strings.Join(segments[1:], "/")
	}
	return ""
}

func splitPath(path string) []string {
	path = strings.Trim(path, "/")
	if path == "" {
		return nil
	}
	out := strings.Split(path, "/")
	for i, v := range out {
		out[i] = redactPathSegment(v)
	}
	return out
}

func redactPathSegment(s string) string {
	lowered := strings.ToLower(s)
	if strings.Contains(lowered, "token") || strings.Contains(lowered, "secret") || strings.Contains(lowered, "password") {
		return "redacted"
	}
	return s
}

func redactError(msg string) string {
	lowered := strings.ToLower(msg)
	if strings.Contains(lowered, "token") || strings.Contains(lowered, "secret") || strings.Contains(lowered, "password") {
		return "redacted"
	}
	return msg
}
