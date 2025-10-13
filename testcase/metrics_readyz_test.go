package testcase

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"kubeop/internal/metrics"
)

func TestObserveReadyzFailureNormalizesReason(t *testing.T) {
	metrics.ResetReadyzFailures()

	metrics.ObserveReadyzFailure("  Db Down  ")
	metrics.ObserveReadyzFailure("")
	metrics.ObserveReadyzFailure("   ")

	if got := testutil.ToFloat64(metrics.ReadyzFailures.WithLabelValues("db down")); got != 1 {
		t.Fatalf("expected normalized reason 'db down' to have count 1, got %v", got)
	}
	if got := testutil.ToFloat64(metrics.ReadyzFailures.WithLabelValues("unknown")); got != 2 {
		t.Fatalf("expected empty reasons to collapse to 'unknown' count 2, got %v", got)
	}
}
