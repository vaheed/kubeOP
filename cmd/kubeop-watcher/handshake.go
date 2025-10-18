package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"kubeop/internal/sink"
	"kubeop/internal/watcher/authmanager"
	watcherhandshake "kubeop/internal/watcher/handshake"
	"kubeop/internal/watcher/readiness"
)

const (
	handshakeMaxBackoff   = 15 * time.Second
	handshakeInitialDelay = time.Second
	handshakeInterval     = 30 * time.Second
	handshakeTimeout      = 10 * time.Second
)

func startHandshakeLoop(ctx context.Context, cfg watcherConfig, status *readiness.Tracker, queue *eventQueue, s *sink.Sink, auth *authmanager.Manager, logger *zap.Logger) {
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
			if auth != nil {
				if err := auth.WaitReady(ctx); err != nil {
					if logger != nil {
						logger.Warn("handshake wait failed", zap.Error(err))
					}
					status.RecordHandshakeFailure(err)
					return
				}
			}
			token := ""
			if auth != nil {
				token = auth.AccessToken()
			}
			_, err := watcherhandshake.Perform(ctx, client, cfg.HandshakeURL, token, cfg.ClusterID)
			if err != nil {
				status.RecordHandshakeFailure(err)
				if logger != nil {
					logger.Warn("handshake failed", zap.Error(err), zap.Duration("backoff", backoff))
				}
				if auth != nil && isUnauthorized(err) {
					refreshCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
					if err := auth.ForceRefresh(refreshCtx); err != nil && logger != nil {
						logger.Warn("forced token refresh after handshake failure", zap.Error(err))
					}
					cancel()
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
				status.RecordFlushFailure(err)
				if logger != nil {
					logger.Warn("flush queued events failed", zap.Error(err))
				}
			} else {
				status.RecordFlushSuccess(time.Now())
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

func isUnauthorized(err error) bool {
	if err == nil {
		return false
	}
	var httpErr interface{ Error() string }
	httpErr = err
	if strings.Contains(strings.ToLower(httpErr.Error()), "status 401") || strings.Contains(strings.ToLower(httpErr.Error()), "status 403") {
		return true
	}
	return false
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
