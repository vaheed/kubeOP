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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

func TestSinkRejectsHTTPWithoutAllowInsecure(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	if _, err := sink.New(sink.Config{URL: ts.URL, Token: "token"}, zap.NewNop()); err == nil {
		t.Fatalf("expected error for http URL without AllowInsecure")
	}
}

func TestSinkAllowsHTTPWhenInsecureEnabled(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	snk, err := sink.New(sink.Config{URL: ts.URL, Token: "token", AllowInsecure: true}, zap.NewNop())
	if err != nil {
		t.Fatalf("expected sink to allow http when AllowInsecure enabled: %v", err)
	}
	if snk == nil {
		t.Fatalf("expected sink instance")
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

type queueStub struct {
	mu     sync.Mutex
	stored [][]sink.Event
}

func (q *queueStub) Store(events []sink.Event) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	copyEvents := make([]sink.Event, len(events))
	copy(copyEvents, events)
	q.stored = append(q.stored, copyEvents)
	return nil
}

func (q *queueStub) count() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.stored)
}

func TestSinkPersistsFailedBatches(t *testing.T) {
	ts := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()

	client := ts.Client()
	client.Timeout = time.Second

	queue := &queueStub{}
	snk, err := sink.New(sink.Config{
		URL:             ts.URL,
		Token:           "token",
		BatchMax:        1,
		BatchWindow:     5 * time.Millisecond,
		HTTPTimeout:     time.Second,
		HTTPClient:      client,
		PersistentQueue: queue,
	}, zap.NewNop())
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan struct{})
	go func() {
		snk.Run(ctx)
		close(done)
	}()

	event := sink.Event{ClusterID: "cluster", EventType: "Added", Kind: "Pod", Namespace: "ns", Name: "pod", Summary: "create", DedupKey: "uid#1"}
	if ok := snk.Enqueue(event); !ok {
		t.Fatalf("expected enqueue to succeed")
	}

	waitForQueueCount(t, queue, 1)

	if ok := snk.Enqueue(event); !ok {
		t.Fatalf("expected re-enqueue to succeed after persistence")
	}

	cancel()
	snk.Stop()
	<-done
}

func waitForQueueCount(t *testing.T, q *queueStub, expected int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		if q.count() >= expected {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timeout waiting for queue count %d (got %d)", expected, q.count())
		}
		time.Sleep(10 * time.Millisecond)
	}
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
