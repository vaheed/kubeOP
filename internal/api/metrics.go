package api

import (
    "net/http"
    "time"

    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promhttp"
    collectors "github.com/prometheus/client_golang/prometheus/collectors"
)

var (
    reqTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
        Namespace: "kubeop",
        Subsystem: "api",
        Name:      "requests_total",
        Help:      "Total HTTP requests",
    }, []string{"method", "path", "code"})

    reqDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
        Namespace: "kubeop",
        Subsystem: "api",
        Name:      "request_duration_seconds",
        Help:      "HTTP request duration",
        Buckets:   prometheus.DefBuckets,
    }, []string{"method", "path"})
)

func init() {
    // Default process/go collectors; ignore AlreadyRegistered to avoid panics when other frameworks register them
    _ = prometheus.Register(collectors.NewGoCollector())
    _ = prometheus.Register(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
    _ = prometheus.Register(reqTotal)
    _ = prometheus.Register(reqDuration)
}

type statusWriter struct {
    http.ResponseWriter
    code int
}

func (w *statusWriter) WriteHeader(statusCode int) {
    w.code = statusCode
    w.ResponseWriter.WriteHeader(statusCode)
}

func instrument(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        sw := &statusWriter{ResponseWriter: w, code: 200}
        start := time.Now()
        next.ServeHTTP(sw, r)
        path := r.URL.Path
        reqTotal.WithLabelValues(r.Method, path, http.StatusText(sw.code)).Inc()
        reqDuration.WithLabelValues(r.Method, path).Observe(time.Since(start).Seconds())
    })
}

var promhttpHandler = promhttp.Handler()

// PromHandler exposes the Prometheus handler for reuse in other binaries
func PromHandler() http.Handler { return promhttpHandler }
