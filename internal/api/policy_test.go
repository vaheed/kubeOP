package api

import (
    "bytes"
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "os"
    "testing"

    dbpkg "github.com/vaheed/kubeop/internal/db"
    "github.com/vaheed/kubeop/internal/kms"
    "github.com/vaheed/kubeop/internal/logging"
)

// Test manager-side image allowlist on app create
func Test_Policy_ImageAllowlist_OnCreate(t *testing.T) {
    dsn := os.Getenv("KUBEOP_DB_URL")
    if dsn == "" { t.Skip("no db url") }
    db, err := dbpkg.Connect(dsn)
    if err != nil { t.Skipf("db: %v", err) }
    if err := db.Migrate(context.Background()); err != nil { t.Fatalf("migrate: %v", err) }
    key := make([]byte, 32)
    for i := range key { key[i] = byte(1) }
    enc, _ := kms.New(key)
    lg := logging.New("test")
    s := New(lg, db, enc, false, nil)

    // seed tenant+project via store directly
    tnt, err := s.store.CreateTenant(context.Background(), "t1")
    if err != nil { t.Fatalf("tenant: %v", err) }
    prj, err := s.store.CreateProject(context.Background(), tnt.ID, "p1")
    if err != nil { t.Fatalf("project: %v", err) }

    // Set policy to only allow docker.io
    os.Setenv("KUBEOP_IMAGE_ALLOWLIST", "docker.io")
    t.Cleanup(func(){ os.Unsetenv("KUBEOP_IMAGE_ALLOWLIST") })

    srv := httptest.NewServer(s.Router())
    defer srv.Close()

    body := map[string]any{"projectID": prj.ID, "name": "a1", "image": "evil.io/forbidden:latest"}
    b, _ := json.Marshal(body)
    resp, err := http.Post(srv.URL+"/v1/apps", "application/json", bytes.NewReader(b))
    if err != nil { t.Fatalf("post: %v", err) }
    if resp.StatusCode == 200 { t.Fatalf("expected policy denial, got 200") }
}

func Test_IsRegistryAllowed_Env(t *testing.T) {
    s := &Server{}
    os.Setenv("KUBEOP_IMAGE_ALLOWLIST", "docker.io,ghcr.io")
    t.Cleanup(func(){ os.Unsetenv("KUBEOP_IMAGE_ALLOWLIST") })
    if !s.isRegistryAllowed(context.Background(), "docker.io") {
        t.Fatalf("expected docker.io allowed")
    }
    if s.isRegistryAllowed(context.Background(), "evil.io") {
        t.Fatalf("expected evil.io denied")
    }
}

func Test_ImageHost_Parse(t *testing.T) {
    if got := imageHost("nginx:1.25"); got != "docker.io" { t.Fatalf("want docker.io, got %s", got) }
    if got := imageHost("ghcr.io/org/app:1"); got != "ghcr.io" { t.Fatalf("want ghcr.io, got %s", got) }
}
