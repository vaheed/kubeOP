package watcher_test

import (
	"errors"
	"testing"
	"time"

	"kubeop/internal/watcher/readiness"
)

func TestReadinessTrackerHandshakeAndDelivery(t *testing.T) {
	tracker := readiness.New()

	if ready, _, detail := tracker.HandshakeStatus(time.Minute); ready {
		t.Fatalf("expected handshake to be pending, got ready with detail %q", detail)
	}

	tracker.RecordHandshakeFailure(errors.New("handshake failed"))
	if ready, _, detail := tracker.HandshakeStatus(time.Minute); ready || detail != "handshake failed" {
		t.Fatalf("expected handshake failure detail, got ready=%v detail=%q", ready, detail)
	}

	now := time.Now()
	tracker.RecordHandshakeSuccess(now)
	if ready, ts, detail := tracker.HandshakeStatus(time.Minute); !ready || ts.IsZero() || detail != "" {
		t.Fatalf("expected handshake success, got ready=%v ts=%v detail=%q", ready, ts, detail)
	}

	if deliverOK, ts, detail := tracker.DeliveryStatus(time.Minute); !deliverOK || (!ts.IsZero() && detail != "") {
		t.Fatalf("expected delivery to default ready, got ok=%v ts=%v detail=%q", deliverOK, ts, detail)
	}

	tracker.RecordFlushFailure(errors.New("ingest down"))
	if deliverOK, _, detail := tracker.DeliveryStatus(time.Minute); deliverOK || detail != "ingest down" {
		t.Fatalf("expected delivery failure detail, got ok=%v detail=%q", deliverOK, detail)
	}

	tracker.RecordFlushSuccess(now.Add(2 * time.Second))
	if deliverOK, ts, detail := tracker.DeliveryStatus(time.Minute); !deliverOK || ts.IsZero() || detail != "" {
		t.Fatalf("expected delivery success, got ok=%v ts=%v detail=%q", deliverOK, ts, detail)
	}

	stale := time.Now().Add(-2 * time.Minute)
	tracker.RecordFlushSuccess(stale)
	if deliverOK, ts, detail := tracker.DeliveryStatus(time.Minute); deliverOK || detail != "delivery stale" || !ts.Equal(stale) {
		t.Fatalf("expected stale delivery, got ok=%v ts=%v detail=%q", deliverOK, ts, detail)
	}

	if deliverOK, ts, detail := tracker.DeliveryStatus(0); !deliverOK || !ts.Equal(stale) || detail != "" {
		t.Fatalf("expected delivery to be considered ready without max age, got ok=%v ts=%v detail=%q", deliverOK, ts, detail)
	}
}
