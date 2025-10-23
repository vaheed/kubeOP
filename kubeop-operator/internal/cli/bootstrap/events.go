package bootstrap

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AuditEvent represents a minimal CloudEvent payload used for bootstrap actions.
type AuditEvent struct {
	SpecVersion     string      `json:"specversion"`
	Type            string      `json:"type"`
	Source          string      `json:"source"`
	ID              string      `json:"id"`
	Time            time.Time   `json:"time"`
	DataContentType string      `json:"datacontenttype"`
	Data            interface{} `json:"data,omitempty"`
}

// EventSink emits CloudEvents to the desired backend.
type EventSink interface {
	Emit(ctx context.Context, event AuditEvent) error
}

// JSONEventSink writes CloudEvents as JSON objects to the configured writer.
type JSONEventSink struct {
	writer io.Writer
	mu     sync.Mutex
}

// NewJSONEventSink constructs an event sink that writes to the provided writer.
func NewJSONEventSink(w io.Writer) (*JSONEventSink, error) {
	if w == nil {
		return nil, fmt.Errorf("writer is required")
	}
	return &JSONEventSink{writer: w}, nil
}

// Emit serialises the event to JSON and writes it to the sink.
func (s *JSONEventSink) Emit(_ context.Context, event AuditEvent) error {
	if s == nil {
		return fmt.Errorf("event sink is nil")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if event.SpecVersion == "" {
		event.SpecVersion = "1.0"
	}
	if event.ID == "" {
		event.ID = uuid.NewString()
	}
	if event.Time.IsZero() {
		event.Time = time.Now().UTC()
	}
	if event.DataContentType == "" {
		event.DataContentType = "application/json"
	}
	enc := json.NewEncoder(s.writer)
	enc.SetEscapeHTML(false)
	return enc.Encode(event)
}

// NewAuditEvent creates a baseline CloudEvent for bootstrap operations.
func NewAuditEvent(eventType, source string, data interface{}) AuditEvent {
	return AuditEvent{
		SpecVersion:     "1.0",
		Type:            eventType,
		Source:          source,
		ID:              uuid.NewString(),
		Time:            time.Now().UTC(),
		DataContentType: "application/json",
		Data:            data,
	}
}
