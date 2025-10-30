package e2e

import (
    "encoding/json"
    "io"
    "net/http"
    "os"
    "os/exec"
    "path/filepath"
    "testing"
    "time"
)

func Test_ApiSmoke(t *testing.T) {
    if os.Getenv("KUBEOP_E2E") == "" {
        t.Skip("KUBEOP_E2E not set")
    }
    // Expect compose to be running DB+manager (handled by CI step)
    artifacts := os.Getenv("ARTIFACTS_DIR")
    if artifacts == "" { artifacts = "artifacts" }
    smokeDir := filepath.Join(artifacts, "smoke")
    _ = os.MkdirAll(smokeDir, 0o755)
    t.Cleanup(func(){
        exec.Command("bash", "-lc", "docker compose ps > "+artifacts+"/compose-ps.txt 2>&1").Run()
        exec.Command("bash", "-lc", "docker compose logs manager > "+artifacts+"/manager.log 2>&1").Run()
        exec.Command("bash", "-lc", "docker compose logs db > "+artifacts+"/db.log 2>&1").Run()
    })

    base := "http://localhost:18080"
    // readiness loop (up to 120s)
    readyPath := filepath.Join(smokeDir, "ready.txt")
    deadline := time.Now().Add(120 * time.Second)
    for {
        if time.Now().After(deadline) {
            os.WriteFile(readyPath, []byte("timeout waiting for /readyz\n"), 0o644)
            t.Fatalf("manager not ready within 120s")
        }
        resp, err := http.Get(base+"/readyz")
        if err == nil {
            b, _ := io.ReadAll(resp.Body)
            resp.Body.Close()
            os.WriteFile(readyPath, append([]byte(resp.Status+"\n"), b...), 0o644)
            if resp.StatusCode == 200 { break }
        } else {
            os.WriteFile(readyPath, []byte(err.Error()+"\n"), 0o644)
        }
        time.Sleep(2*time.Second)
    }

    // health
    if resp, err := http.Get(base+"/healthz"); err == nil {
        b, _ := io.ReadAll(resp.Body); resp.Body.Close()
        os.WriteFile(filepath.Join(smokeDir, "health.txt"), append([]byte(resp.Status+"\n"), b...), 0o644)
        if resp.StatusCode != 200 { t.Fatalf("/healthz status %d", resp.StatusCode) }
    } else {
        os.WriteFile(filepath.Join(smokeDir, "health.txt"), []byte(err.Error()+"\n"), 0o644)
        t.Fatalf("/healthz error: %v", err)
    }

    // version
    if resp, err := http.Get(base+"/version"); err == nil {
        var v map[string]any
        b, _ := io.ReadAll(resp.Body); resp.Body.Close()
        os.WriteFile(filepath.Join(smokeDir, "version.json"), b, 0o644)
        _ = json.Unmarshal(b, &v)
        if svc, _ := v["service"].(string); svc != "manager" { t.Fatalf("unexpected service: %v", svc) }
    } else {
        os.WriteFile(filepath.Join(smokeDir, "version.json"), []byte(err.Error()+"\n"), 0o644)
        t.Fatalf("/version error: %v", err)
    }

    // metrics
    if resp, err := http.Get(base+"/metrics"); err == nil {
        b, _ := io.ReadAll(resp.Body); resp.Body.Close()
        os.WriteFile(filepath.Join(smokeDir, "metrics.txt"), b, 0o644)
        if resp.StatusCode != 200 { t.Fatalf("/metrics status %d", resp.StatusCode) }
    } else {
        os.WriteFile(filepath.Join(smokeDir, "metrics.txt"), []byte(err.Error()+"\n"), 0o644)
        t.Fatalf("/metrics error: %v", err)
    }

    // openapi
    if resp, err := http.Get(base+"/openapi.json"); err == nil {
        b, _ := io.ReadAll(resp.Body); resp.Body.Close()
        os.WriteFile(filepath.Join(smokeDir, "openapi.json"), b, 0o644)
        if resp.StatusCode != 200 { t.Fatalf("/openapi.json status %d", resp.StatusCode) }
    } else {
        os.WriteFile(filepath.Join(smokeDir, "openapi.json"), []byte(err.Error()+"\n"), 0o644)
        t.Fatalf("/openapi.json error: %v", err)
    }
}
