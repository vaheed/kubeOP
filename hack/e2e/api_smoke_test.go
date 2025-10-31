package e2e

import (
    "bytes"
    "io"
    "net/http"
    "os"
    "testing"
    "time"
)

// Test_ApiSmoke verifies the manager API is up when compose is running.
func Test_ApiSmoke(t *testing.T) {
    if os.Getenv("KUBEOP_E2E") == "" {
        t.Skip("KUBEOP_E2E not set")
    }
    httpc := &http.Client{Timeout: 5 * time.Second}
    // healthz
    resp, err := httpc.Get("http://localhost:18080/healthz")
    if err != nil { t.Fatalf("healthz: %v", err) }
    io.Copy(io.Discard, resp.Body); resp.Body.Close()
    if resp.StatusCode != 200 { t.Fatalf("healthz status: %d", resp.StatusCode) }
    // version
    resp, err = httpc.Get("http://localhost:18080/version")
    if err != nil { t.Fatalf("version: %v", err) }
    b, _ := io.ReadAll(resp.Body); resp.Body.Close()
    if !bytes.Contains(b, []byte("manager")) {
        t.Fatalf("version body unexpected: %s", string(b))
    }
    // openapi.json exists
    resp, err = httpc.Get("http://localhost:18080/openapi.json")
    if err != nil { t.Fatalf("openapi: %v", err) }
    io.Copy(io.Discard, resp.Body); resp.Body.Close()
}

