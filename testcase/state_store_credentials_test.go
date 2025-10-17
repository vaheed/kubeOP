package testcase

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"kubeop/internal/state"
)

func TestStoreCredentialsLifecycle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "state.db")
	st, err := state.Open(path)
	if err != nil {
		t.Fatalf("state.Open: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	creds := state.Credentials{
		WatcherID:      "watcher-1",
		AccessToken:    "access",
		AccessExpires:  time.Now().Add(time.Minute).UTC().Round(time.Second),
		RefreshToken:   "refresh",
		RefreshExpires: time.Now().Add(time.Hour).UTC().Round(time.Second),
	}
	if err := st.SaveCredentials(creds); err != nil {
		t.Fatalf("SaveCredentials: %v", err)
	}
	loaded, ok, err := st.LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if !ok {
		t.Fatalf("expected credentials present")
	}
	if loaded.WatcherID != creds.WatcherID || loaded.AccessToken != creds.AccessToken || loaded.RefreshToken != creds.RefreshToken {
		t.Fatalf("unexpected credentials: %+v", loaded)
	}
	if err := st.ClearCredentials(); err != nil {
		t.Fatalf("ClearCredentials: %v", err)
	}
	_, ok, err = st.LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials after clear: %v", err)
	}
	if ok {
		t.Fatalf("expected credentials cleared")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state file missing: %v", err)
	}
}
