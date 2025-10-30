package e2e

import (
    "context"
    "encoding/json"
    "io"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "testing"
    "time"
)

// Test_ClusterEndpoints validates operator in-cluster endpoints via port-forward
// It hits: /healthz (8082), /readyz (8082), /version (8083), /metrics (8081)
func Test_ClusterEndpoints(t *testing.T) {
    if os.Getenv("KUBEOP_E2E") == "" {
        t.Skip("KUBEOP_E2E not set")
    }

    artifacts := os.Getenv("ARTIFACTS_DIR")
    if artifacts == "" { artifacts = "artifacts" }
    outDir := filepath.Join(artifacts, "cluster")
    _ = os.MkdirAll(outDir, 0o755)

    // Ensure operator is ready
    deadline := time.Now().Add(2 * time.Minute)
    for {
        if time.Now().After(deadline) {
            t.Fatalf("operator not ready within 2m")
        }
        if out, _ := exec.Command("bash", "-lc", "kubectl -n kubeop-system get deploy/kubeop-operator -o jsonpath='{.status.availableReplicas}'").CombinedOutput(); string(out) == "1" {
            break
        }
        time.Sleep(3 * time.Second)
    }

    // Start port-forward for metrics:8081, health:8082, version:8083
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    pf := exec.CommandContext(ctx, "bash", "-lc", "kubectl -n kubeop-system port-forward deploy/kubeop-operator 18081:8081 18082:8082 18083:8083")
    pf.Stdout = io.Discard
    pf.Stderr = io.Discard
    if err := pf.Start(); err != nil {
        t.Fatalf("port-forward: %v", err)
    }
    // Give port-forward some time
    time.Sleep(2 * time.Second)

    httpc := &http.Client{Timeout: 5 * time.Second}
    // healthz
    if resp, err := httpc.Get("http://127.0.0.1:18082/healthz"); err == nil {
        b, _ := io.ReadAll(resp.Body); resp.Body.Close()
        os.WriteFile(filepath.Join(outDir, "health.txt"), append([]byte(resp.Status+"\n"), b...), 0o644)
        if resp.StatusCode != 200 { t.Fatalf("/healthz=%d", resp.StatusCode) }
    } else {
        os.WriteFile(filepath.Join(outDir, "health.txt"), []byte(err.Error()), 0o644)
        t.Fatalf("/healthz error: %v", err)
    }

    // readyz
    if resp, err := httpc.Get("http://127.0.0.1:18082/readyz"); err == nil {
        b, _ := io.ReadAll(resp.Body); resp.Body.Close()
        os.WriteFile(filepath.Join(outDir, "ready.txt"), append([]byte(resp.Status+"\n"), b...), 0o644)
        if resp.StatusCode != 200 { t.Fatalf("/readyz=%d", resp.StatusCode) }
    } else {
        os.WriteFile(filepath.Join(outDir, "ready.txt"), []byte(err.Error()), 0o644)
        t.Fatalf("/readyz error: %v", err)
    }

    // version
    if resp, err := httpc.Get("http://127.0.0.1:18083/version"); err == nil {
        b, _ := io.ReadAll(resp.Body); resp.Body.Close()
        os.WriteFile(filepath.Join(outDir, "version.json"), b, 0o644)
        var v map[string]any
        _ = json.Unmarshal(b, &v)
        if svc, _ := v["service"].(string); svc != "operator" { t.Fatalf("unexpected service: %v", svc) }
    } else {
        os.WriteFile(filepath.Join(outDir, "version.json"), []byte(err.Error()), 0o644)
        t.Fatalf("/version error: %v", err)
    }

    // metrics
    if resp, err := httpc.Get("http://127.0.0.1:18081/metrics"); err == nil {
        b, _ := io.ReadAll(resp.Body); resp.Body.Close()
        os.WriteFile(filepath.Join(outDir, "metrics.txt"), b, 0o644)
        if resp.StatusCode != 200 { t.Fatalf("/metrics=%d", resp.StatusCode) }
    } else {
        os.WriteFile(filepath.Join(outDir, "metrics.txt"), []byte(err.Error()), 0o644)
        t.Fatalf("/metrics error: %v", err)
    }
}

