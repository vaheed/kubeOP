package main

import (
	"sync"
	"time"
)

type readinessTracker struct {
	mu            sync.RWMutex
	storeReady    bool
	lastHandshake time.Time
	lastErr       string
}

func newReadinessTracker() *readinessTracker {
	return &readinessTracker{}
}

func (r *readinessTracker) MarkStoreReady() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.storeReady = true
	r.mu.Unlock()
}

func (r *readinessTracker) StoreReady() bool {
	if r == nil {
		return false
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.storeReady
}

func (r *readinessTracker) RecordHandshakeSuccess(ts time.Time) {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.lastHandshake = ts
	r.lastErr = ""
	r.mu.Unlock()
}

func (r *readinessTracker) RecordHandshakeFailure(err error) {
	if r == nil {
		return
	}
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	r.mu.Lock()
	r.lastErr = msg
	r.mu.Unlock()
}

func (r *readinessTracker) HandshakeStatus(maxAge time.Duration) (bool, time.Time, string) {
	if r == nil {
		return false, time.Time{}, ""
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	last := r.lastHandshake
	err := r.lastErr
	if last.IsZero() {
		if err == "" {
			err = "handshake pending"
		}
		return false, last, err
	}
	if err != "" {
		return false, last, err
	}
	if maxAge > 0 && time.Since(last) > maxAge {
		if err == "" {
			err = "handshake stale"
		}
		return false, last, err
	}
	return true, last, ""
}
