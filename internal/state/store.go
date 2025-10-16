package state

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	bolt "go.etcd.io/bbolt"
)

const (
	resourceVersionBucket = "resource_versions"
	eventQueueBucket      = "event_queue"
)

// Store persists watcher checkpoints (resource versions) so informer streams can
// resume without replaying the entire history on restart.
type Store struct {
	db   *bolt.DB
	path string
	mu   sync.RWMutex
}

// Open initialises the state store at the provided path, creating parent
// directories as required.
func Open(path string) (*Store, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("state path is required")
	}
	abspath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve state path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(abspath), 0o750); err != nil {
		return nil, fmt.Errorf("ensure state directory: %w", err)
	}
	db, err := bolt.Open(abspath, 0o600, nil)
	if err != nil {
		return nil, fmt.Errorf("open state store: %w", err)
	}
	s := &Store{db: db, path: abspath}
	if err := s.ensureBuckets(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) ensureBuckets() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Update(func(tx *bolt.Tx) error {
		if _, err := tx.CreateBucketIfNotExists([]byte(resourceVersionBucket)); err != nil {
			return fmt.Errorf("create resource version bucket: %w", err)
		}
		if _, err := tx.CreateBucketIfNotExists([]byte(eventQueueBucket)); err != nil {
			return fmt.Errorf("create event queue bucket: %w", err)
		}
		return nil
	})
}

// Close flushes and closes the underlying BoltDB handle.
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}

// GetResourceVersion returns the last persisted resourceVersion for the
// provided kind. When unset, an empty string is returned.
func (s *Store) GetResourceVersion(kind string) (string, error) {
	if s == nil {
		return "", errors.New("state store is nil")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var value string
	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(resourceVersionBucket))
		if bucket == nil {
			return fmt.Errorf("bucket %s missing", resourceVersionBucket)
		}
		b := bucket.Get([]byte(kind))
		if len(b) == 0 {
			value = ""
			return nil
		}
		value = string(b)
		return nil
	})
	return value, err
}

// SetResourceVersion stores the latest processed resourceVersion for the given
// kind. Empty resource versions are ignored to avoid clobbering previously
// recorded checkpoints when Kubernetes emits tombstones without metadata.
func (s *Store) SetResourceVersion(kind, resourceVersion string) error {
	if s == nil {
		return errors.New("state store is nil")
	}
	if strings.TrimSpace(kind) == "" {
		return errors.New("kind is required")
	}
	if strings.TrimSpace(resourceVersion) == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(resourceVersionBucket))
		if bucket == nil {
			return fmt.Errorf("bucket %s missing", resourceVersionBucket)
		}
		if err := bucket.Put([]byte(kind), []byte(resourceVersion)); err != nil {
			return fmt.Errorf("persist resource version: %w", err)
		}
		return nil
	})
}

// Path exposes the resolved absolute path on disk. Intended for diagnostics.
func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// QueuedEvent represents a persisted watcher event awaiting delivery.
type QueuedEvent struct {
	ID      uint64
	Payload []byte
}

// EnqueueEvents appends the provided payloads to the durable event queue.
func (s *Store) EnqueueEvents(payloads [][]byte) error {
	if s == nil {
		return errors.New("state store is nil")
	}
	if len(payloads) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(eventQueueBucket))
		if bucket == nil {
			return fmt.Errorf("bucket %s missing", eventQueueBucket)
		}
		for _, payload := range payloads {
			seq, err := bucket.NextSequence()
			if err != nil {
				return fmt.Errorf("next sequence: %w", err)
			}
			key := make([]byte, 8)
			binary.BigEndian.PutUint64(key, seq)
			if err := bucket.Put(key, payload); err != nil {
				return fmt.Errorf("enqueue event: %w", err)
			}
		}
		return nil
	})
}

// PeekEvents returns up to limit queued events without removing them from the queue.
func (s *Store) PeekEvents(limit int) ([]QueuedEvent, error) {
	if s == nil {
		return nil, errors.New("state store is nil")
	}
	if limit <= 0 {
		limit = 200
	}
	events := make([]QueuedEvent, 0, limit)
	s.mu.RLock()
	defer s.mu.RUnlock()
	err := s.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(eventQueueBucket))
		if bucket == nil {
			return fmt.Errorf("bucket %s missing", eventQueueBucket)
		}
		cursor := bucket.Cursor()
		for k, v := cursor.First(); k != nil && len(events) < limit; k, v = cursor.Next() {
			id := binary.BigEndian.Uint64(k)
			buf := make([]byte, len(v))
			copy(buf, v)
			events = append(events, QueuedEvent{ID: id, Payload: buf})
		}
		return nil
	})
	return events, err
}

// DeleteQueuedEvents removes the provided event IDs from the queue.
func (s *Store) DeleteQueuedEvents(ids []uint64) error {
	if s == nil {
		return errors.New("state store is nil")
	}
	if len(ids) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(eventQueueBucket))
		if bucket == nil {
			return fmt.Errorf("bucket %s missing", eventQueueBucket)
		}
		key := make([]byte, 8)
		for _, id := range ids {
			binary.BigEndian.PutUint64(key, id)
			if err := bucket.Delete(key); err != nil {
				return fmt.Errorf("delete queued event: %w", err)
			}
		}
		return nil
	})
}
