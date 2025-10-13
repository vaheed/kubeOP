package middleware

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"kubeop/internal/logging"
)

// AccessLog records structured request/response details for every HTTP call.
func AccessLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := middleware.GetReqID(r.Context())
		if reqID == "" {
			reqID = uuid.NewString()
			ctx := context.WithValue(r.Context(), middleware.RequestIDKey, reqID)
			r = r.Clone(ctx)
		}
		ww := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		w.Header().Set("X-Request-Id", reqID)

		start := time.Now()
		next.ServeHTTP(ww, r)

		latency := time.Since(start)
		logFields := []zap.Field{
			zap.String("request_id", reqID),
			zap.String("method", r.Method),
			zap.String("path", r.URL.Path),
			zap.Int("status", ww.status),
			zap.Float64("latency_ms", float64(latency.Microseconds())/1000.0),
			zap.String("remote_ip", remoteIP(r)),
			zap.String("user_agent", redactUserAgent(r.UserAgent())),
			zap.Int64("bytes_in", bytesIn(r)),
			zap.Int64("bytes_out", ww.written),
			zap.String("tenant_id", tenantID(r)),
			zap.String("user_id", userID(r)),
		}
		if ww.errMsg != "" {
			logFields = append(logFields, zap.String("err", ww.errMsg))
		}

		logger := logging.L()
		if isProbe(r.URL.Path) {
			logger.Debug("http_request", logFields...)
			return
		}
		if ww.status >= http.StatusBadRequest {
			logger.Warn("http_request", logFields...)
			return
		}
		logger.Info("http_request", logFields...)
	})
}

func remoteIP(r *http.Request) string {
	if ip := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); ip != "" {
		parts := strings.Split(ip, ",")
		return strings.TrimSpace(parts[0])
	}
	if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
		return ip
	}
	host := r.RemoteAddr
	if idx := strings.LastIndex(host, ":"); idx > -1 {
		return host[:idx]
	}
	return host
}

func bytesIn(r *http.Request) int64 {
	if r.ContentLength > 0 {
		return r.ContentLength
	}
	return 0
}

func tenantID(r *http.Request) string {
	if v := r.Context().Value(ctxTenantID{}); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func userID(r *http.Request) string { return UserIDFromContext(r.Context()) }

// UserIDFromContext exposes the annotated user ID for reuse by other packages.
func UserIDFromContext(ctx context.Context) string {
        if ctx == nil {
                return ""
        }
        if v := ctx.Value(ctxUserID{}); v != nil {
                if s, ok := v.(string); ok {
                        return s
                }
        }
        return ""
}

func redactUserAgent(ua string) string {
	if strings.Contains(strings.ToLower(ua), "token") {
		return "redacted"
	}
	return ua
}

func isProbe(path string) bool {
	switch path {
	case "/healthz", "/readyz":
		return true
	}
	return false
}

type responseWriter struct {
	http.ResponseWriter
	status  int
	written int64
	errMsg  string
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.written += int64(n)
	if err != nil {
		w.errMsg = err.Error()
	}
	return n, err
}

func (w *responseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("http.Hijacker not supported")
}

func (w *responseWriter) Push(target string, opts *http.PushOptions) error {
	if p, ok := w.ResponseWriter.(http.Pusher); ok {
		return p.Push(target, opts)
	}
	return http.ErrNotSupported
}

type ctxTenantID struct{}
type ctxUserID struct{}

// WithTenantID annotates the request context so middleware can log the tenant.
func WithTenantID(r *http.Request, tenant string) *http.Request {
	ctx := context.WithValue(r.Context(), ctxTenantID{}, tenant)
	return r.Clone(ctx)
}

// WithUserID annotates the request context so middleware can log the user.
func WithUserID(r *http.Request, user string) *http.Request {
	ctx := context.WithValue(r.Context(), ctxUserID{}, user)
	return r.Clone(ctx)
}
