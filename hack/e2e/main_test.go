package e2e

import (
    "context"
    "crypto/rand"
    "fmt"
    "io"
    "net"
    "net/http"
    "os"
    "os/exec"
    "testing"
    "time"

    "github.com/vaheed/kubeop/internal/api"
    "github.com/vaheed/kubeop/internal/db"
    "github.com/vaheed/kubeop/internal/kms"
    "github.com/vaheed/kubeop/internal/logging"
)

// TestMain provides a shared bootstrap for all E2E tests: ensures DB is up and
// starts the local manager binary bound to :18080 with dev-friendly settings.
func TestMain(m *testing.M) {
    if os.Getenv("KUBEOP_E2E") == "" {
        // run normally if E2E not requested
        os.Exit(m.Run())
    }
    artifacts := os.Getenv("ARTIFACTS_DIR")
    if artifacts == "" { artifacts = "artifacts" }
    _ = os.MkdirAll(artifacts, 0o755)

    // Compose lives at repo root; derive paths relative to hack/e2e
    root := "../.."
    compose := "docker compose -f " + root + "/docker-compose.yml"
    envFile := root + "/.env"; if _, err := os.Stat(envFile); err != nil { envFile = root + "/env.example" }
    // Ensure any compose-managed manager is stopped to free port 18080
    _ = exec.Command("bash", "-lc", compose+" --env-file "+envFile+" stop manager >/dev/null 2>&1 || true").Run()
    _ = exec.Command("bash", "-lc", "docker rm -f kubeop-manager >/dev/null 2>&1 || true").Run()
    // Start DB via compose (idempotent)
    _ = exec.Command("bash", "-lc", compose+" --env-file "+envFile+" up -d db").Run()
    // Wait for DB port to accept connections (<= 60s)
    if !waitForTCP("127.0.0.1", 5432, 60*time.Second) {
        fmt.Fprintln(os.Stderr, "[e2e] DB did not become ready on :5432 in time")
        os.Exit(1)
    }

    // Start in-process manager bound to :18080 with dev-friendly config
    os.Setenv("KUBEOP_DB_URL", "postgres://kubeop:kubeop@localhost:5432/kubeop?sslmode=disable")
    os.Setenv("KUBEOP_DEV_INSECURE", "true")
    os.Setenv("KUBEOP_REQUIRE_AUTH", "false")
    // Connect DB
    d, err := db.Connect(os.Getenv("KUBEOP_DB_URL"))
    if err != nil { fmt.Fprintln(os.Stderr, "[e2e] db connect:", err); os.Exit(1) }
    if err := d.Ping(context.Background()); err != nil { fmt.Fprintln(os.Stderr, "[e2e] db ping:", err); os.Exit(1) }
    // Ephemeral KMS key
    key := make([]byte, 32)
    if _, err := rand.Read(key); err != nil { fmt.Fprintln(os.Stderr, "[e2e] rng:", err); os.Exit(1) }
    enc, err := kms.New(key)
    if err != nil { fmt.Fprintln(os.Stderr, "[e2e] kms:", err); os.Exit(1) }
    lg := logging.New("manager-e2e")
    srv := api.New(lg, d, enc, false, nil)
    srv.MustMigrate(context.Background())
    ctx, cancel := context.WithCancel(context.Background())
    go func() { _ = srv.Start(ctx, ":18080") }()
    // Wait for readiness (<= 90s)
    deadline := time.Now().Add(90 * time.Second)
    for time.Now().Before(deadline) {
        resp, err := http.Get("http://localhost:18080/readyz")
        if err == nil {
            io.Copy(io.Discard, resp.Body)
            resp.Body.Close()
            if resp.StatusCode == 200 { break }
        }
        time.Sleep(2 * time.Second)
    }
    // Final check
    if resp, err := http.Get("http://localhost:18080/readyz"); err != nil || resp.StatusCode != 200 {
        fmt.Fprintln(os.Stderr, "[e2e] Manager failed to become ready; check DB service and port conflicts on :18080")
        if resp != nil { io.Copy(io.Discard, resp.Body); resp.Body.Close() }
        os.Exit(1)
    }

    code := m.Run()
    cancel()
    os.Exit(code)
}

// waitForTCP returns true if a TCP connection to host:port succeeds before timeout.
func waitForTCP(host string, port int, timeout time.Duration) bool {
    deadline := time.Now().Add(timeout)
    addr := fmt.Sprintf("%s:%d", host, port)
    for time.Now().Before(deadline) {
        if c, err := net.DialTimeout("tcp", addr, 2*time.Second); err == nil {
            _ = c.Close()
            return true
        }
        time.Sleep(1 * time.Second)
    }
    return false
}
