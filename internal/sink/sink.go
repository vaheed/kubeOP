package sink

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"kubeop/internal/metrics"
)

const (
	defaultBatchMax        = 200
	minBatchMax            = 1
	defaultBatchWindow     = time.Second
	defaultHTTPTimeout     = 15 * time.Second
	maxBackoff             = 30 * time.Second
	initialBackoff         = 250 * time.Millisecond
	maxDedupEntries        = 8192
	dedupRetention         = time.Hour
	minimumCompressionSize = 8 * 1024
)

// Event is the normalised payload delivered to kubeOP.
type Event struct {
	ClusterID string            `json:"cluster_id"`
	EventType string            `json:"event_type"`
	Kind      string            `json:"kind"`
	Namespace string            `json:"namespace"`
	Name      string            `json:"name"`
	Labels    map[string]string `json:"labels,omitempty"`
	Summary   string            `json:"summary"`
	DedupKey  string            `json:"dedup_key"`
}

// Config configures batching and delivery behaviour for the sink.
type Config struct {
	URL             string
	Token           string
	BatchMax        int
	BatchWindow     time.Duration
	HTTPTimeout     time.Duration
	UserAgent       string
	HTTPClient      *http.Client
	PersistentQueue PersistentQueue
	AllowInsecure   bool
	OnUnauthorized  func()
}

// Enqueuer describes the subset of sink behaviour used by the watcher manager.
type Enqueuer interface {
	Enqueue(Event) bool
}

// PersistentQueue captures the behaviour required to durably store events when
// delivery to the API is unavailable.
type PersistentQueue interface {
	Store([]Event) error
}

type Sink struct {
	client  *http.Client
	logger  *zap.Logger
	cfg     Config
	queueMu sync.Mutex
	queue   []Event
	trigger chan struct{}
	stopped chan struct{}
	dedupe  *deduper
	ready   atomic.Bool
	persist PersistentQueue
	token   atomic.Value
}

// New constructs a sink with sane defaults and validates the remote URL.
func New(cfg Config, logger *zap.Logger) (*Sink, error) {
	if cfg.URL == "" {
		return nil, errors.New("KUBEOP_EVENTS_URL is required")
	}
	parsed, err := url.Parse(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("parse kubeOP events URL: %w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "https":
	case "http":
		if !cfg.AllowInsecure {
			return nil, fmt.Errorf("kubeOP events URL must use https (got %s)", parsed.Scheme)
		}
	default:
		return nil, fmt.Errorf("kubeOP events URL must be http or https (got %s)", parsed.Scheme)
	}
	if cfg.BatchMax <= 0 {
		cfg.BatchMax = defaultBatchMax
	}
	if cfg.BatchMax > defaultBatchMax {
		cfg.BatchMax = defaultBatchMax
	}
	if cfg.BatchWindow <= 0 {
		cfg.BatchWindow = defaultBatchWindow
	}
	if cfg.HTTPTimeout <= 0 {
		cfg.HTTPTimeout = defaultHTTPTimeout
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = "kubeop-watcher"
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	client := cfg.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: cfg.HTTPTimeout}
	} else if cfg.HTTPTimeout > 0 {
		client.Timeout = cfg.HTTPTimeout
	}
	s := &Sink{
		client:  client,
		logger:  logger,
		cfg:     cfg,
		trigger: make(chan struct{}, 1),
		stopped: make(chan struct{}),
		dedupe:  newDeduper(maxDedupEntries, dedupRetention),
		persist: cfg.PersistentQueue,
	}
	s.token.Store(strings.TrimSpace(cfg.Token))
	return s, nil
}

// Ready reports whether the sink has successfully delivered at least one batch.
func (s *Sink) Ready() bool {
	return s.ready.Load()
}

// Stop waits for the background loop to exit once the context passed to Run is cancelled.
func (s *Sink) Stop() {
	<-s.stopped
}

// Enqueue adds an event to the delivery queue, returning true when the event
// was accepted. Events missing a deduplication key are dropped.
func (s *Sink) Enqueue(event Event) bool {
	if event.DedupKey == "" {
		s.logger.Debug("drop event: missing dedup key", zap.String("kind", event.Kind), zap.String("name", event.Name))
		metrics.ObserveDrop("missing_dedup_key")
		return false
	}
	if !s.dedupe.Add(event.DedupKey) {
		metrics.ObserveDrop("duplicate")
		return false
	}
	s.queueMu.Lock()
	s.queue = append(s.queue, event)
	depth := len(s.queue)
	s.queueMu.Unlock()
	metrics.SetQueueDepth(depth)
	metrics.ObserveEnqueue(event.Kind, event.EventType)
	if depth >= s.cfg.BatchMax {
		s.signal()
	}
	return true
}

func (s *Sink) signal() {
	select {
	case s.trigger <- struct{}{}:
	default:
	}
}

// Run processes the queue, batching and delivering events until the context is
// cancelled.
func (s *Sink) Run(ctx context.Context) {
	defer close(s.stopped)
	ticker := time.NewTicker(s.cfg.BatchWindow)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			s.flushOnShutdown()
			return
		case <-s.trigger:
		case <-ticker.C:
		}
		s.processQueue(ctx)
	}
}

func (s *Sink) processQueue(ctx context.Context) {
	for {
		batch := s.dequeue()
		if len(batch) == 0 {
			return
		}
		if err := s.sendWithRetry(ctx, batch); err != nil {
			s.logger.Warn("failed to deliver batch", zap.Int("size", len(batch)), zap.Error(err))
			if s.persist != nil {
				if err := s.persist.Store(batch); err != nil {
					s.logger.Error("failed to persist batch", zap.Int("size", len(batch)), zap.Error(err))
					s.requeue(batch)
				} else {
					s.logger.Info("persisted batch for later delivery", zap.Int("size", len(batch)))
					s.resetDedup(batch)
				}
			} else {
				s.requeue(batch)
			}
			return
		}
	}
}

func (s *Sink) dequeue() []Event {
	s.queueMu.Lock()
	defer s.queueMu.Unlock()
	if len(s.queue) == 0 {
		return nil
	}
	limit := s.cfg.BatchMax
	if limit <= 0 {
		limit = defaultBatchMax
	}
	if len(s.queue) < limit {
		limit = len(s.queue)
	}
	batch := make([]Event, limit)
	copy(batch, s.queue[:limit])
	s.queue = append([]Event{}, s.queue[limit:]...)
	metrics.SetQueueDepth(len(s.queue))
	return batch
}

func (s *Sink) requeue(events []Event) {
	s.queueMu.Lock()
	defer s.queueMu.Unlock()
	if len(events) == 0 {
		return
	}
	buf := make([]Event, 0, len(events)+len(s.queue))
	buf = append(buf, events...)
	buf = append(buf, s.queue...)
	s.queue = buf
	metrics.SetQueueDepth(len(s.queue))
	s.signal()
}

func (s *Sink) resetDedup(events []Event) {
	if s == nil || len(events) == 0 {
		return
	}
	for _, event := range events {
		s.dedupe.Remove(event.DedupKey)
	}
}

func (s *Sink) flushOnShutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		batch := s.dequeue()
		if len(batch) == 0 {
			return
		}
		if err := s.postBatch(ctx, batch); err != nil {
			s.logger.Warn("dropping batch during shutdown", zap.Int("size", len(batch)), zap.Error(err))
		}
	}
}

func (s *Sink) sendWithRetry(ctx context.Context, events []Event) error {
	if len(events) == 0 {
		return nil
	}
	attempt := 0
	maxAttempts := 0
	if s.persist != nil {
		maxAttempts = 1
	}
	for {
		err := s.postBatch(ctx, events)
		if err == nil {
			s.ready.Store(true)
			metrics.ObserveBatch("success")
			metrics.SetLastSuccessfulPush(time.Now().Unix())
			return nil
		}
		metrics.ObserveBatch("failure")
		attempt++
		if maxAttempts > 0 && attempt >= maxAttempts {
			s.logger.Warn("aborting batch after failed attempt", zap.Int("attempt", attempt), zap.Error(err))
			return fmt.Errorf("aborted after %d attempt(s): %w", attempt, err)
		}
		backoff := initialBackoff * time.Duration(1<<uint(min(attempt-1, 6)))
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
		s.logger.Warn("retrying batch", zap.Int("attempt", attempt), zap.Duration("backoff", backoff), zap.Error(err))
		select {
		case <-ctx.Done():
			return fmt.Errorf("aborted after %d attempts: %w", attempt, err)
		case <-time.After(backoff):
		}
	}
}

func (s *Sink) postBatch(ctx context.Context, events []Event) error {
	body, encoding, err := encodeEvents(events)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.URL, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token := s.currentToken(); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	req.Header.Set("User-Agent", s.cfg.UserAgent)
	if encoding != "" {
		req.Header.Set("Content-Encoding", encoding)
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		if cb := s.cfg.OnUnauthorized; cb != nil {
			cb()
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return nil
}

// SetToken updates the bearer token used for subsequent deliveries.
func (s *Sink) SetToken(token string) {
	if s == nil {
		return
	}
	s.token.Store(strings.TrimSpace(token))
}

func (s *Sink) currentToken() string {
	if s == nil {
		return ""
	}
	if tok, ok := s.token.Load().(string); ok {
		return tok
	}
	return ""
}

// DeliverBatch pushes the provided events immediately, bypassing the in-memory queue.
func (s *Sink) DeliverBatch(ctx context.Context, events []Event) error {
	if s == nil {
		return errors.New("sink is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return s.sendWithRetry(ctx, events)
}

func encodeEvents(events []Event) (io.Reader, string, error) {
	buf, err := json.Marshal(events)
	if err != nil {
		return nil, "", fmt.Errorf("marshal events: %w", err)
	}
	if len(buf) == 0 {
		return bytes.NewReader([]byte("[]")), "", nil
	}
	if len(buf) > minimumCompressionSize {
		var gz bytes.Buffer
		zw := gzip.NewWriter(&gz)
		if _, err := zw.Write(buf); err != nil {
			zw.Close()
			return nil, "", fmt.Errorf("gzip payload: %w", err)
		}
		if err := zw.Close(); err != nil {
			return nil, "", fmt.Errorf("finalise gzip payload: %w", err)
		}
		return bytes.NewReader(gz.Bytes()), "gzip", nil
	}
	return bytes.NewReader(buf), "", nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

type deduper struct {
	mu      sync.Mutex
	entries map[string]time.Time
	order   []dedupeEntry
	limit   int
	ttl     time.Duration
}

type dedupeEntry struct {
	key string
	ts  time.Time
}

func newDeduper(limit int, ttl time.Duration) *deduper {
	if limit < minBatchMax {
		limit = minBatchMax
	}
	if ttl <= 0 {
		ttl = time.Hour
	}
	return &deduper{
		entries: make(map[string]time.Time),
		order:   make([]dedupeEntry, 0, limit),
		limit:   limit,
		ttl:     ttl,
	}
}

func (d *deduper) Add(key string) bool {
	if key == "" {
		return false
	}
	now := time.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	if ts, ok := d.entries[key]; ok {
		if now.Sub(ts) <= d.ttl {
			d.entries[key] = now
			d.order = append(d.order, dedupeEntry{key: key, ts: now})
			d.pruneLocked(now)
			return false
		}
	}
	d.entries[key] = now
	d.order = append(d.order, dedupeEntry{key: key, ts: now})
	d.pruneLocked(now)
	return true
}

func (d *deduper) Remove(key string) {
	if d == nil || key == "" {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.entries, key)
	cleaned := d.order[:0]
	for _, entry := range d.order {
		if entry.key == key {
			continue
		}
		cleaned = append(cleaned, entry)
	}
	d.order = cleaned
}

func (d *deduper) pruneLocked(now time.Time) {
	cutoff := now.Add(-d.ttl)
	cleaned := d.order[:0]
	for _, entry := range d.order {
		ts, ok := d.entries[entry.key]
		if !ok {
			continue
		}
		if ts != entry.ts {
			continue
		}
		if ts.Before(cutoff) || len(d.entries) > d.limit {
			delete(d.entries, entry.key)
			continue
		}
		cleaned = append(cleaned, entry)
	}
	d.order = cleaned
}
