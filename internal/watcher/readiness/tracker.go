package readiness

import (
	"sync"
	"time"
)

// Tracker records watcher readiness state for health probes.
type Tracker struct {
	mu               sync.RWMutex
	storeReady       bool
	lastHandshake    time.Time
	lastHandshakeErr string
	lastFlush        time.Time
	lastFlushErr     string
}

// New constructs a Tracker with zeroed state.
func New() *Tracker {
	return &Tracker{}
}

// MarkStoreReady flags the persistent state store as initialised.
func (t *Tracker) MarkStoreReady() {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.storeReady = true
	t.mu.Unlock()
}

// StoreReady reports whether the state store has been initialised.
func (t *Tracker) StoreReady() bool {
	if t == nil {
		return false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.storeReady
}

// RecordHandshakeSuccess stores the timestamp of the last successful handshake.
func (t *Tracker) RecordHandshakeSuccess(ts time.Time) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.lastHandshake = ts
	t.lastHandshakeErr = ""
	t.mu.Unlock()
}

// RecordHandshakeFailure captures the most recent handshake error.
func (t *Tracker) RecordHandshakeFailure(err error) {
	if t == nil {
		return
	}
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	t.mu.Lock()
	t.lastHandshakeErr = msg
	t.mu.Unlock()
}

// HandshakeStatus reports whether the last handshake succeeded within maxAge.
func (t *Tracker) HandshakeStatus(maxAge time.Duration) (bool, time.Time, string) {
	if t == nil {
		return false, time.Time{}, ""
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	last := t.lastHandshake
	err := t.lastHandshakeErr
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

// RecordFlushFailure captures the most recent queue flush error.
func (t *Tracker) RecordFlushFailure(err error) {
	if t == nil {
		return
	}
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	t.mu.Lock()
	t.lastFlushErr = msg
	t.mu.Unlock()
}

// RecordFlushSuccess records the timestamp of the most recent successful flush.
func (t *Tracker) RecordFlushSuccess(ts time.Time) {
	if t == nil {
		return
	}
	t.mu.Lock()
	t.lastFlush = ts
	t.lastFlushErr = ""
	t.mu.Unlock()
}

// DeliveryStatus reports whether queued events have been flushed recently.
func (t *Tracker) DeliveryStatus(maxAge time.Duration) (bool, time.Time, string) {
	if t == nil {
		return false, time.Time{}, ""
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	if t.lastFlushErr != "" {
		return false, t.lastFlush, t.lastFlushErr
	}
	if !t.lastFlush.IsZero() && maxAge > 0 && time.Since(t.lastFlush) > maxAge {
		return false, t.lastFlush, "delivery stale"
	}
	return true, t.lastFlush, ""
}
