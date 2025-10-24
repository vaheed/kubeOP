package metrics

import (
	"context"
	"sync"
	"time"
)

// FakeProvider returns preseeded usage snapshots for tests.
type FakeProvider struct {
	mu    sync.Mutex
	usage map[time.Time][]UsageSample
}

// NewFakeProvider constructs a FakeProvider.
func NewFakeProvider() *FakeProvider {
	return &FakeProvider{usage: make(map[time.Time][]UsageSample)}
}

// SetUsage stores samples for a window.
func (f *FakeProvider) SetUsage(window time.Time, samples []UsageSample) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.usage == nil {
		f.usage = make(map[time.Time][]UsageSample)
	}
	truncated := window.UTC().Truncate(time.Hour)
	cloned := make([]UsageSample, len(samples))
	copy(cloned, samples)
	f.usage[truncated] = cloned
}

// CollectUsage returns samples for the requested window.
func (f *FakeProvider) CollectUsage(_ context.Context, window time.Time) ([]UsageSample, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.usage == nil {
		return nil, nil
	}
	truncated := window.UTC().Truncate(time.Hour)
	samples := f.usage[truncated]
	out := make([]UsageSample, len(samples))
	copy(out, samples)
	return out, nil
}
