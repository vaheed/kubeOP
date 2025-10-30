package e2e

import (
    "bytes"
    "encoding/json"
    "fmt"
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
    type epResult struct{
        Name string `json:"name"`
        URL string `json:"url"`
        Status int `json:"status"`
        DurationMS int64 `json:"durationMs"`
        Error string `json:"error,omitempty"`
        Bytes int `json:"bytes"`
    }
    summary := struct{
        StartedAt string `json:"startedAt"`
        ReadyInMS int64 `json:"readyInMs"`
        Endpoints []epResult `json:"endpoints"`
        Passed bool `json:"passed"`
    }{StartedAt: time.Now().UTC().Format(time.RFC3339)}
    // Ensure summary is written even if test fails early
    defer func(){
        b, _ := json.MarshalIndent(summary, "", "  ")
        _ = os.WriteFile(filepath.Join(smokeDir, "summary.json"), b, 0o644)
        // also write a human readable summary
        var buf bytes.Buffer
        fmt.Fprintf(&buf, "Smoke Summary\nStarted: %s\nReadyInMS: %d\n\n", summary.StartedAt, summary.ReadyInMS)
        for _, r := range summary.Endpoints {
            fmt.Fprintf(&buf, "%-10s %3d %5dms %s %s\n", r.Name, r.Status, r.DurationMS, r.URL, r.Error)
        }
        fmt.Fprintf(&buf, "\nPASS=%v\n", summary.Passed)
        _ = os.WriteFile(filepath.Join(smokeDir, "summary.txt"), buf.Bytes(), 0o644)
    }()

    // readiness loop (up to 120s)
    readyPath := filepath.Join(smokeDir, "ready.txt")
    deadline := time.Now().Add(120 * time.Second)
    var readyStart = time.Now()
    readyOK := false
    for {
        if time.Now().After(deadline) {
            _ = os.WriteFile(readyPath, []byte("timeout waiting for /readyz\n"), 0o644)
            break
        }
        resp, err := http.Get(base+"/readyz")
        if err == nil {
            b, _ := io.ReadAll(resp.Body)
            resp.Body.Close()
            _ = os.WriteFile(readyPath, append([]byte(resp.Status+"\n"), b...), 0o644)
            if resp.StatusCode == 200 { readyOK = true; break }
        } else {
            _ = os.WriteFile(readyPath, []byte(err.Error()+"\n"), 0o644)
        }
        time.Sleep(2*time.Second)
    }
    if readyOK { summary.ReadyInMS = time.Since(readyStart).Milliseconds() }

    fail := false
    // helper to hit endpoint and record
    hit := func(name, url string, want int, save string){
        t.Helper()
        t1 := time.Now()
        resp, err := http.Get(url)
        if err != nil {
            summary.Endpoints = append(summary.Endpoints, epResult{Name:name, URL:url, Status:0, DurationMS: time.Since(t1).Milliseconds(), Error: err.Error()})
            fail = true
            return
        }
        b, _ := io.ReadAll(resp.Body); resp.Body.Close()
        _ = os.WriteFile(filepath.Join(smokeDir, save), b, 0o644)
        if resp.StatusCode != want { fail = true }
        summary.Endpoints = append(summary.Endpoints, epResult{Name:name, URL:url, Status:resp.StatusCode, DurationMS: time.Since(t1).Milliseconds(), Bytes: len(b)})
    }

    if readyOK {
        hit("healthz", base+"/healthz", 200, "health.txt")
        hit("version", base+"/version", 200, "version.json")
        hit("metrics", base+"/metrics", 200, "metrics.txt")
        hit("openapi", base+"/openapi.json", 200, "openapi.json")
        hit("tenants", base+"/v1/tenants", 200, "tenants.json")
    } else {
        fail = true
    }
    summary.Passed = !fail
    if fail { t.Fail() }
}
