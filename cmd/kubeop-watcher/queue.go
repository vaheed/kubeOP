package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"go.uber.org/zap"

	"kubeop/internal/sink"
	"kubeop/internal/state"
)

type queuedEvent struct {
	ID    uint64
	Event sink.Event
}

type eventQueue struct {
	store  *state.Store
	logger *zap.Logger
}

func newEventQueue(store *state.Store, logger *zap.Logger) *eventQueue {
	return &eventQueue{store: store, logger: logger}
}

func (q *eventQueue) Store(events []sink.Event) error {
	if q == nil || q.store == nil {
		return errors.New("event queue not initialised")
	}
	if len(events) == 0 {
		return nil
	}
	payloads := make([][]byte, 0, len(events))
	for _, evt := range events {
		data, err := json.Marshal(evt)
		if err != nil {
			return fmt.Errorf("marshal event: %w", err)
		}
		payloads = append(payloads, data)
	}
	return q.store.EnqueueEvents(payloads)
}

func (q *eventQueue) Load(limit int) ([]queuedEvent, error) {
	if q == nil || q.store == nil {
		return nil, errors.New("event queue not initialised")
	}
	records, err := q.store.PeekEvents(limit)
	if err != nil {
		return nil, err
	}
	events := make([]queuedEvent, 0, len(records))
	for _, rec := range records {
		var evt sink.Event
		if err := json.Unmarshal(rec.Payload, &evt); err != nil {
			return nil, fmt.Errorf("decode queued event %d: %w", rec.ID, err)
		}
		events = append(events, queuedEvent{ID: rec.ID, Event: evt})
	}
	return events, nil
}

func (q *eventQueue) Delete(ids []uint64) error {
	if q == nil || q.store == nil {
		return errors.New("event queue not initialised")
	}
	return q.store.DeleteQueuedEvents(ids)
}
