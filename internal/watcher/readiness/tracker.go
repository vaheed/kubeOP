package readiness

import (
	"sync"
	"time"
)

// HandshakeReport summarises the last handshake attempts.
type HandshakeReport struct {
	Ready    bool
	Fresh    bool
	Last     time.Time
	Detail   string
	Degraded bool
	Ever     bool
}

// DeliveryReport summarises queue flush attempts.
type DeliveryReport struct {
	Healthy  bool
	Last     time.Time
	Detail   string
	Degraded bool
	Ever     bool
}

// Tracker records watcher readiness state for health probes.
type Tracker struct {
	mu                   sync.RWMutex
	storeReady           bool
	lastHandshake        time.Time
	lastHandshakeErr     string
	lastHandshakeFailure time.Time
	lastFlush            time.Time
	lastFlushErr         string
	lastFlushFailure     time.Time
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
	t.lastHandshakeFailure = time.Time{}
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
	t.lastHandshakeFailure = time.Now()
	t.mu.Unlock()
}

// HandshakeStatus summarises the last handshake attempts within maxAge.
func (t *Tracker) HandshakeStatus(maxAge time.Duration) HandshakeReport {
	if t == nil {
		return HandshakeReport{}
	}
	t.mu.RLock()
	defer t.mu.RUnlock()

	last := t.lastHandshake
	ever := !last.IsZero()
	err := t.lastHandshakeErr
	failureAfterSuccess := ever && t.lastHandshakeFailure.After(last)

	report := HandshakeReport{
		Last: last,
		Ever: ever,
	}

	if !ever {
		if err == "" {
			err = "handshake pending"
		}
		report.Detail = err
		report.Degraded = true
		return report
	}

	report.Fresh = true
	report.Ready = true

	if maxAge > 0 && time.Since(last) > maxAge {
		if err == "" {
			err = "handshake stale"
		}
		report.Ready = false
		report.Fresh = false
		report.Degraded = true
		report.Detail = err
		return report
	}

	if failureAfterSuccess {
		if err == "" {
			err = "last handshake attempt failed"
		}
		report.Degraded = true
		report.Detail = err
	}

	return report
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
	t.lastFlushFailure = time.Now()
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
	t.lastFlushFailure = time.Time{}
	t.mu.Unlock()
}

// DeliveryStatus summarises the last queue flush attempts.
func (t *Tracker) DeliveryStatus(maxAge time.Duration) DeliveryReport {
	if t == nil {
		return DeliveryReport{}
	}
	t.mu.RLock()
	defer t.mu.RUnlock()

	last := t.lastFlush
	ever := !last.IsZero()
	err := t.lastFlushErr
	failureAfterSuccess := ever && t.lastFlushFailure.After(last)

	report := DeliveryReport{
		Last: last,
		Ever: ever,
	}

	if !ever {
		if err == "" {
			err = "flush pending"
		}
		report.Detail = err
		report.Degraded = true
		return report
	}

	report.Healthy = true

	if maxAge > 0 && time.Since(last) > maxAge {
		if err == "" {
			err = "delivery stale"
		}
		report.Healthy = false
		report.Degraded = true
		report.Detail = err
		return report
	}

	if failureAfterSuccess {
		if err == "" {
			err = "last flush attempt failed"
		}
		report.Degraded = true
		report.Detail = err
	}

	return report
}
