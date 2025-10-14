package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	bolt "go.etcd.io/bbolt"
)

const resourceVersionBucket = "resource_versions"

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
		_, err := tx.CreateBucketIfNotExists([]byte(resourceVersionBucket))
		if err != nil {
			return fmt.Errorf("create resource version bucket: %w", err)
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
