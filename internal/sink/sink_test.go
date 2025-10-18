package sink

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestDeliverBatchRetriesWithUpdatedToken(t *testing.T) {
	t.Parallel()

	var attempts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		switch attempts {
		case 1:
			if got := r.Header.Get("Authorization"); got != "Bearer initial" {
				t.Fatalf("expected initial token, got %q", got)
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		case 2:
			if got := r.Header.Get("Authorization"); got != "Bearer rotated" {
				t.Fatalf("expected rotated token, got %q", got)
			}
			w.WriteHeader(http.StatusAccepted)
		default:
			t.Fatalf("unexpected attempt %d", attempts)
		}
	}))
	t.Cleanup(srv.Close)

	client := srv.Client()
	client.Timeout = time.Second

	var sinkInstance *Sink
	cfg := Config{
		URL:           srv.URL,
		Token:         "initial",
		HTTPClient:    client,
		AllowInsecure: true,
	}
	cfg.OnUnauthorized = func(context.Context) error {
		sinkInstance.SetToken("rotated")
		return nil
	}

	s, err := New(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}
	sinkInstance = s

	event := Event{DedupKey: "test"}
	if err := s.DeliverBatch(context.Background(), []Event{event}); err != nil {
		t.Fatalf("deliver batch: %v", err)
	}

	if attempts != 2 {
		t.Fatalf("expected 2 attempts, got %d", attempts)
	}
}

func TestDeliverBatchUsesTokenProvider(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt := attempts.Add(1)
		switch attempt {
		case 1:
			if got := r.Header.Get("Authorization"); got != "Bearer initial-provider" {
				t.Fatalf("expected provider token on first attempt, got %q", got)
			}
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		case 2:
			if got := r.Header.Get("Authorization"); got != "Bearer rotated-provider" {
				t.Fatalf("expected rotated provider token, got %q", got)
			}
			w.WriteHeader(http.StatusAccepted)
		default:
			t.Fatalf("unexpected attempt %d", attempt)
		}
	}))
	t.Cleanup(server.Close)

	client := server.Client()
	client.Timeout = time.Second

	token := atomic.Value{}
	token.Store("initial-provider")

	cfg := Config{
		URL:           server.URL,
		HTTPClient:    client,
		AllowInsecure: true,
		TokenProvider: func() string { return token.Load().(string) },
	}
	cfg.OnUnauthorized = func(context.Context) error {
		token.Store("rotated-provider")
		return nil
	}

	s, err := New(cfg, zap.NewNop())
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}

	evt := Event{DedupKey: "pod#1"}
	if err := s.DeliverBatch(context.Background(), []Event{evt}); err != nil {
		t.Fatalf("deliver batch: %v", err)
	}

	if got := attempts.Load(); got != 2 {
		t.Fatalf("expected 2 attempts, got %d", got)
	}
}
