package e2e

import (
    "fmt"
    "io"
    "net"
    "net/http"
    "os"
    "os/exec"
    "testing"
    "time"
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

    // Ensure any compose-managed manager is stopped to free port 18080
    _ = exec.Command("bash", "-lc", "docker compose stop manager >/dev/null 2>&1 || true").Run()
    // Start DB via compose (idempotent)
    _ = exec.Command("bash", "-lc", "docker compose up -d db").Run()
    // Wait for DB port to accept connections (<= 60s)
    if !waitForTCP("127.0.0.1", 5432, 60*time.Second) {
        fmt.Fprintln(os.Stderr, "[e2e] DB did not become ready on :5432 in time")
        os.Exit(1)
    }

    // Build manager if needed
    _ = exec.Command("bash", "-lc", "make build").Run()

    // Start local manager
    mgrCmd := exec.Command("bash", "-lc", "./bin/manager")
    mgrCmd.Env = append(os.Environ(),
        "KUBEOP_DB_URL=postgres://kubeop:kubeop@localhost:5432/kubeop?sslmode=disable",
        "KUBEOP_DEV_INSECURE=true",
        "KUBEOP_REQUIRE_AUTH=false",
        "KUBEOP_HTTP_ADDR=:18080",
    )
    lf, _ := os.Create(artifacts+"/manager-local.log")
    mgrCmd.Stdout = lf
    mgrCmd.Stderr = lf
    _ = mgrCmd.Start()
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
        fmt.Fprintln(os.Stderr, "[e2e] Manager failed to become ready; see artifacts/manager-local.log")
        if resp != nil { io.Copy(io.Discard, resp.Body); resp.Body.Close() }
        os.Exit(1)
    }

    code := m.Run()

    _ = mgrCmd.Process.Kill()
    _ = lf.Close()
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
