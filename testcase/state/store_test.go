package state_test

import (
	"path/filepath"
	"testing"

	statepkg "kubeop/internal/state"
)

func TestStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	s, err := statepkg.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	if err := s.SetResourceVersion("Pod", "12345"); err != nil {
		t.Fatalf("set resource version: %v", err)
	}
	got, err := s.GetResourceVersion("Pod")
	if err != nil {
		t.Fatalf("get resource version: %v", err)
	}
	if got != "12345" {
		t.Fatalf("unexpected resource version: %s", got)
	}

	// Empty writes should be ignored and not clear the stored value.
	if err := s.SetResourceVersion("Pod", ""); err != nil {
		t.Fatalf("set empty resource version: %v", err)
	}
	still, err := s.GetResourceVersion("Pod")
	if err != nil {
		t.Fatalf("get resource version after empty set: %v", err)
	}
	if still != "12345" {
		t.Fatalf("expected resource version to remain, got %s", still)
	}
}

func TestEventQueuePersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "events.db")
	s, err := statepkg.Open(path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })

	payloads := [][]byte{
		[]byte(`{"cluster_id":"c1","dedup_key":"a"}`),
		[]byte(`{"cluster_id":"c1","dedup_key":"b"}`),
	}
	if err := s.EnqueueEvents(payloads); err != nil {
		t.Fatalf("enqueue events: %v", err)
	}

	records, err := s.PeekEvents(10)
	if err != nil {
		t.Fatalf("peek events: %v", err)
	}
	if len(records) != len(payloads) {
		t.Fatalf("expected %d queued events, got %d", len(payloads), len(records))
	}
	if string(records[0].Payload) != string(payloads[0]) {
		t.Fatalf("expected first payload %s, got %s", payloads[0], records[0].Payload)
	}

	if err := s.DeleteQueuedEvents([]uint64{records[0].ID}); err != nil {
		t.Fatalf("delete queued event: %v", err)
	}

	remaining, err := s.PeekEvents(10)
	if err != nil {
		t.Fatalf("peek events after delete: %v", err)
	}
	if len(remaining) != 1 {
		t.Fatalf("expected one queued event remaining, got %d", len(remaining))
	}
	if string(remaining[0].Payload) != string(payloads[1]) {
		t.Fatalf("expected remaining payload %s, got %s", payloads[1], remaining[0].Payload)
	}

	if err := s.DeleteQueuedEvents([]uint64{remaining[0].ID}); err != nil {
		t.Fatalf("delete last event: %v", err)
	}
	empty, err := s.PeekEvents(10)
	if err != nil {
		t.Fatalf("peek events final: %v", err)
	}
	if len(empty) != 0 {
		t.Fatalf("expected queue empty, got %d events", len(empty))
	}
}
