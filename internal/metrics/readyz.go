package metrics

import (
	"strings"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	// ReadyzFailures counts readiness probe failures with a reason label.
	ReadyzFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "readyz_failures_total",
			Help: "Total number of readiness probe failures grouped by reason.",
		},
		[]string{"reason"},
	)
)

func init() {
	prometheus.MustRegister(ReadyzFailures)
}

// ObserveReadyzFailure increments the failure counter for the provided reason.
func ObserveReadyzFailure(reason string) {
	r := strings.TrimSpace(strings.ToLower(reason))
	if r == "" {
		r = "unknown"
	}
	ReadyzFailures.WithLabelValues(r).Inc()
}

// ResetReadyzFailures clears all readiness failure label values. Intended for tests.
func ResetReadyzFailures() {
	ReadyzFailures.Reset()
}
