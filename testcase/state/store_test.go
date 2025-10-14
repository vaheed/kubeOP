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
