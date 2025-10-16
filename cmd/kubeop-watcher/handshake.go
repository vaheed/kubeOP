package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"kubeop/internal/sink"
	watcherhandshake "kubeop/internal/watcher/handshake"
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
			_, err := watcherhandshake.Perform(ctx, client, cfg.HandshakeURL, cfg.Token, cfg.ClusterID)
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
