package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"kubeop/internal/sink"
)

const (
	handshakeMaxBackoff   = 15 * time.Second
	handshakeInitialDelay = time.Second
	handshakeInterval     = 30 * time.Second
	handshakeTimeout      = 10 * time.Second
)

func startHandshakeLoop(ctx context.Context, cfg watcherConfig, status *readinessTracker, queue *eventQueue, s *sink.Sink, logger *zap.Logger) {
	if s == nil {
		return
	}
	go func() {
		client := &http.Client{Timeout: handshakeTimeout}
		backoff := handshakeInitialDelay
		for {
			if ctx.Err() != nil {
				return
			}
			err := performHandshake(ctx, client, cfg.HandshakeURL, cfg.Token, cfg.ClusterID)
			if err != nil {
				status.RecordHandshakeFailure(err)
				if logger != nil {
					logger.Warn("handshake failed", zap.Error(err), zap.Duration("backoff", backoff))
				}
				select {
				case <-ctx.Done():
					return
				case <-time.After(backoff):
				}
				backoff *= 2
				if backoff > handshakeMaxBackoff {
					backoff = handshakeMaxBackoff
				}
				continue
			}
			now := time.Now()
			status.RecordHandshakeSuccess(now)
			if logger != nil {
				logger.Info("handshake succeeded", zap.Time("at", now))
			}
			if err := flushPersistedEvents(ctx, queue, s, cfg.BatchMax, logger); err != nil {
				status.RecordHandshakeFailure(err)
				if logger != nil {
					logger.Warn("flush queued events failed", zap.Error(err))
				}
			}
			backoff = handshakeInitialDelay
			wait := handshakeInterval
			if wait <= 0 {
				wait = 30 * time.Second
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(wait):
			}
		}
	}()
}

func performHandshake(ctx context.Context, client *http.Client, url, token, expectedCluster string) error {
	if client == nil {
		client = &http.Client{Timeout: handshakeTimeout}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		return fmt.Errorf("build handshake request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("handshake request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("read handshake response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		trimmed := strings.TrimSpace(string(body))
		return fmt.Errorf("handshake unexpected status %d: %s", resp.StatusCode, trimmed)
	}
	var payload struct {
		Status    string `json:"status"`
		ClusterID string `json:"cluster_id"`
	}
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			return fmt.Errorf("decode handshake response: %w", err)
		}
	}
	if payload.ClusterID == "" {
		payload.ClusterID = expectedCluster
	}
	if payload.ClusterID == "" {
		return errors.New("handshake response missing cluster_id")
	}
	if expectedCluster != "" && payload.ClusterID != expectedCluster {
		return fmt.Errorf("handshake cluster mismatch: expected %s got %s", expectedCluster, payload.ClusterID)
	}
	return nil
}

func flushPersistedEvents(ctx context.Context, q *eventQueue, s *sink.Sink, batchSize int, logger *zap.Logger) error {
	if q == nil || s == nil {
		return nil
	}
	if batchSize <= 0 || batchSize > 200 {
		batchSize = 200
	}
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		items, err := q.Load(batchSize)
		if err != nil {
			return fmt.Errorf("load queued events: %w", err)
		}
		if len(items) == 0 {
			return nil
		}
		events := make([]sink.Event, len(items))
		ids := make([]uint64, len(items))
		for i, item := range items {
			events[i] = item.Event
			ids[i] = item.ID
		}
		if err := s.DeliverBatch(ctx, events); err != nil {
			return fmt.Errorf("deliver queued events: %w", err)
		}
		if err := q.Delete(ids); err != nil {
			return fmt.Errorf("delete queued events: %w", err)
		}
		if logger != nil {
			logger.Info("flushed queued events", zap.Int("count", len(events)))
		}
	}
}
