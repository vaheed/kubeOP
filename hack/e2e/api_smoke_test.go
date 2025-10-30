package e2e

import (
    "encoding/json"
    "io"
    "net/http"
    "os"
    "os/exec"
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
    _ = os.MkdirAll(artifacts, 0o755)
    t.Cleanup(func(){
        exec.Command("bash", "-lc", "docker compose ps > "+artifacts+"/compose-ps.txt 2>&1").Run()
        exec.Command("bash", "-lc", "docker compose logs manager > "+artifacts+"/manager.log 2>&1").Run()
        exec.Command("bash", "-lc", "docker compose logs db > "+artifacts+"/db.log 2>&1").Run()
    })
    base := "http://localhost:18080"
    deadline := time.Now().Add(90 * time.Second)
    for {
        if time.Now().After(deadline) { t.Fatalf("manager not ready within 90s") }
        resp, err := http.Get(base+"/readyz")
        if err == nil {
            io.Copy(io.Discard, resp.Body); resp.Body.Close()
            if resp.StatusCode == 200 { break }
        }
        time.Sleep(2*time.Second)
    }
    // health
    resp, err := http.Get(base+"/healthz")
    if err != nil || resp.StatusCode != 200 { t.Fatalf("/healthz: %v %d", err, resp.StatusCode) }
    io.Copy(io.Discard, resp.Body); resp.Body.Close()
    // version
    resp, err = http.Get(base+"/version")
    if err != nil { t.Fatalf("version: %v", err) }
    var v map[string]any
    if err := json.NewDecoder(resp.Body).Decode(&v); err != nil { t.Fatalf("decode version: %v", err) }
    resp.Body.Close()
    if v["service"] != "manager" { t.Fatalf("unexpected service: %v", v["service"]) }
}

