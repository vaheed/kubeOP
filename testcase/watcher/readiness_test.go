package watcher_test

import (
	"errors"
	"testing"
	"time"

	"kubeop/internal/watcher/readiness"
)

func TestReadinessTrackerHandshakeAndDelivery(t *testing.T) {
	tracker := readiness.New()

	hs := tracker.HandshakeStatus(time.Minute)
	if hs.Ready || !hs.Degraded || hs.Detail != "handshake pending" {
		t.Fatalf("expected pending handshake to be degraded=false ready=false, got %+v", hs)
	}

	tracker.RecordHandshakeFailure(errors.New("handshake failed"))
	hs = tracker.HandshakeStatus(time.Minute)
	if hs.Ready || !hs.Degraded || hs.Detail != "handshake failed" {
		t.Fatalf("expected handshake failure detail, got %+v", hs)
	}

	now := time.Now()
	tracker.RecordHandshakeSuccess(now)
	hs = tracker.HandshakeStatus(time.Minute)
	if !hs.Ready || hs.Degraded || !hs.Fresh || hs.Detail != "" || !hs.Ever || !hs.Last.Equal(now) {
		t.Fatalf("expected handshake success, got %+v", hs)
	}

	tracker.RecordHandshakeFailure(errors.New("connect refused"))
	hs = tracker.HandshakeStatus(time.Minute)
	if !hs.Ready || !hs.Degraded || hs.Detail != "connect refused" || !hs.Last.Equal(now) {
		t.Fatalf("expected degraded handshake after failure, got %+v", hs)
	}

	dr := tracker.DeliveryStatus(time.Minute)
	if dr.Healthy || !dr.Degraded || dr.Detail != "flush pending" {
		t.Fatalf("expected pending delivery to be degraded, got %+v", dr)
	}

	tracker.RecordFlushFailure(errors.New("ingest down"))
	dr = tracker.DeliveryStatus(time.Minute)
	if dr.Healthy || !dr.Degraded || dr.Detail != "ingest down" {
		t.Fatalf("expected delivery failure detail, got %+v", dr)
	}

	tracker.RecordFlushSuccess(now.Add(2 * time.Second))
	dr = tracker.DeliveryStatus(time.Minute)
	if !dr.Healthy || dr.Degraded || dr.Detail != "" || dr.Last.IsZero() {
		t.Fatalf("expected delivery success, got %+v", dr)
	}

	stale := time.Now().Add(-2 * time.Minute)
	tracker.RecordFlushSuccess(stale)
	dr = tracker.DeliveryStatus(time.Minute)
	if dr.Healthy || !dr.Degraded || dr.Detail != "delivery stale" || !dr.Last.Equal(stale) {
		t.Fatalf("expected stale delivery, got %+v", dr)
	}

	dr = tracker.DeliveryStatus(0)
	if !dr.Healthy || dr.Degraded || dr.Detail != "" || !dr.Last.Equal(stale) {
		t.Fatalf("expected delivery ready without max age, got %+v", dr)
	}
}
