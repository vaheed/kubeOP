package sink_test

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"kubeop/internal/sink"
)

func TestSinkDeliversEvent(t *testing.T) {
	var (
		mu       sync.Mutex
		requests [][]byte
		headers  []http.Header
	)
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read body: %v", err)
		}
		mu.Lock()
		requests = append(requests, body)
		headers = append(headers, r.Header.Clone())
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := ts.Client()
	client.Timeout = time.Second
	snk, err := sink.New(sink.Config{
		URL:         ts.URL,
		Token:       "token",
		BatchMax:    1,
		BatchWindow: 10 * time.Millisecond,
		HTTPTimeout: time.Second,
		HTTPClient:  client,
		UserAgent:   "test",
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		snk.Run(ctx)
		close(done)
	}()

	event := sink.Event{ClusterID: "cluster", EventType: "Added", Kind: "Pod", Namespace: "ns", Name: "pod", Summary: "created", DedupKey: "uid#1"}
	if ok := snk.Enqueue(event); !ok {
		t.Fatalf("expected event to enqueue")
	}

	waitForRequests(t, &mu, &requests, 1)
	cancel()
	snk.Stop()
	<-done

	if len(requests) == 0 {
		t.Fatalf("expected at least 1 request, got %d", len(requests))
	}
	if headers[0].Get("Authorization") != "Bearer token" {
		t.Fatalf("missing auth header")
	}
	var payload []map[string]interface{}
	if err := json.Unmarshal(requests[0], &payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if len(payload) != 1 {
		t.Fatalf("expected one event in payload, got %d", len(payload))
	}
}

func TestSinkDeduplicatesEvents(t *testing.T) {
	var mu sync.Mutex
	count := 0
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		count++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := ts.Client()
	client.Timeout = time.Second
	snk, err := sink.New(sink.Config{
		URL:         ts.URL,
		Token:       "token",
		BatchMax:    1,
		BatchWindow: 10 * time.Millisecond,
		HTTPTimeout: time.Second,
		HTTPClient:  client,
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go snk.Run(ctx)

	event := sink.Event{ClusterID: "cluster", EventType: "Added", Kind: "Pod", Namespace: "ns", Name: "pod", Summary: "created", DedupKey: "uid#1"}
	if ok := snk.Enqueue(event); !ok {
		t.Fatalf("expected enqueue to succeed")
	}
	if ok := snk.Enqueue(event); ok {
		t.Fatalf("duplicate enqueue should be dropped")
	}

	waitForCount(t, &mu, &count, 1)
	cancel()
	snk.Stop()
}

func TestSinkCompressesLargePayloads(t *testing.T) {
	compressed := make(chan bool, 1)
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		compressed <- r.Header.Get("Content-Encoding") == "gzip"
		if r.Header.Get("Content-Encoding") == "gzip" {
			zr, err := gzip.NewReader(r.Body)
			if err != nil {
				t.Errorf("gzip reader: %v", err)
			} else {
				_, _ = io.Copy(io.Discard, zr)
				zr.Close()
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	client := ts.Client()
	client.Timeout = time.Second
	snk, err := sink.New(sink.Config{
		URL:         ts.URL,
		Token:       "token",
		BatchMax:    1,
		BatchWindow: 5 * time.Millisecond,
		HTTPTimeout: time.Second,
		HTTPClient:  client,
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go snk.Run(ctx)

	largeSummary := strings.Repeat("a", 9000)
	event := sink.Event{ClusterID: "cluster", EventType: "Added", Kind: "Pod", Namespace: "ns", Name: "pod", Summary: largeSummary, DedupKey: "uid#1"}
	snk.Enqueue(event)

	select {
	case ok := <-compressed:
		if !ok {
			t.Fatalf("expected gzip encoding")
		}
	case <-time.After(time.Second):
		t.Fatalf("timeout waiting for request")
	}
	cancel()
	snk.Stop()
}

func waitForRequests(t *testing.T, mu *sync.Mutex, requests *[][]byte, expected int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		count := len(*requests)
		mu.Unlock()
		if count >= expected {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for %d requests", expected)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func waitForCount(t *testing.T, mu *sync.Mutex, value *int, expected int) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for {
		mu.Lock()
		count := *value
		mu.Unlock()
		if count >= expected {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for count %d", expected)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
