package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func Test_ApiSmoke(t *testing.T) {
	if os.Getenv("KUBEOP_E2E") == "" {
		t.Skip("KUBEOP_E2E not set")
	}
	requireTool(t, "docker")

	artifacts := os.Getenv("ARTIFACTS_DIR")
	if artifacts == "" {
		artifacts = "artifacts"
	}
	smokeDir := filepath.Join(artifacts, "smoke")
	if err := os.MkdirAll(smokeDir, 0o755); err != nil {
		t.Fatalf("create smoke dir: %v", err)
	}

	recorder := NewResultsRecorder(t, "smoke")

	t.Cleanup(func() {
		_, _ = runCommand(t, "bash", []string{"-lc", "docker compose ps > " + filepath.Join(artifacts, "compose-ps.txt") + " 2>&1"})
		_, _ = runCommand(t, "bash", []string{"-lc", "docker compose logs manager > " + filepath.Join(artifacts, "manager.log") + " 2>&1"})
		_, _ = runCommand(t, "bash", []string{"-lc", "docker compose logs db > " + filepath.Join(artifacts, "db.log") + " 2>&1"})
	})

	base := "http://localhost:18080"
	type epResult struct {
		Name       string `json:"name"`
		URL        string `json:"url"`
		Status     int    `json:"status"`
		DurationMS int64  `json:"durationMs"`
		Error      string `json:"error,omitempty"`
		Bytes      int    `json:"bytes"`
	}
	summary := struct {
		StartedAt string     `json:"startedAt"`
		ReadyInMS int64      `json:"readyInMs"`
		Endpoints []epResult `json:"endpoints"`
		Passed    bool       `json:"passed"`
	}{StartedAt: time.Now().UTC().Format(time.RFC3339)}

	defer func() {
		b, _ := json.MarshalIndent(summary, "", "  ")
		_ = os.WriteFile(filepath.Join(smokeDir, "summary.json"), b, 0o644)
		var buf bytes.Buffer
		fmt.Fprintf(&buf, "Smoke Summary\nStarted: %s\nReadyInMS: %d\n\n", summary.StartedAt, summary.ReadyInMS)
		for _, r := range summary.Endpoints {
			fmt.Fprintf(&buf, "%-10s %3d %5dms %s %s\n", r.Name, r.Status, r.DurationMS, r.URL, r.Error)
		}
		fmt.Fprintf(&buf, "\nPASS=%v\n", summary.Passed)
		_ = os.WriteFile(filepath.Join(smokeDir, "summary.txt"), buf.Bytes(), 0o644)
	}()

	readyPath := filepath.Join(smokeDir, "ready.txt")
	httpc := &http.Client{Timeout: 10 * time.Second}

	readyOK := false
	if err := recorder.Step("wait manager ready", func() (string, error) {
		deadline := time.Now().Add(120 * time.Second)
		start := time.Now()
		attempts := 0
		for {
			attempts++
			if time.Now().After(deadline) {
				_ = os.WriteFile(readyPath, []byte("timeout waiting for /readyz\n"), 0o644)
				return fmt.Sprintf("attempts=%d", attempts), fmt.Errorf("timeout waiting for /readyz")
			}
			resp, err := httpc.Get(base + "/readyz")
			if err == nil {
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				_ = os.WriteFile(readyPath, append([]byte(resp.Status+"\n"), body...), 0o644)
				if resp.StatusCode == http.StatusOK {
					summary.ReadyInMS = time.Since(start).Milliseconds()
					readyOK = true
					return fmt.Sprintf("attempts=%d", attempts), nil
				}
			} else {
				_ = os.WriteFile(readyPath, []byte(err.Error()+"\n"), 0o644)
			}
			time.Sleep(2 * time.Second)
		}
	}); err != nil {
		summary.Passed = false
		t.Fatal(err)
	}

	fail := false
	hit := func(name, url string, want int, save string) {
		stepName := "endpoint:" + name
		if err := recorder.Step(stepName, func() (string, error) {
			t.Helper()
			t1 := time.Now()
			resp, err := httpc.Get(url)
			if err != nil {
				summary.Endpoints = append(summary.Endpoints, epResult{Name: name, URL: url, Status: 0, DurationMS: time.Since(t1).Milliseconds(), Error: err.Error()})
				return "", err
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			_ = os.WriteFile(filepath.Join(smokeDir, save), body, 0o644)
			duration := time.Since(t1).Milliseconds()
			entry := epResult{Name: name, URL: url, Status: resp.StatusCode, DurationMS: duration, Bytes: len(body)}
			if resp.StatusCode != want {
				entry.Error = fmt.Sprintf("want %d", want)
				summary.Endpoints = append(summary.Endpoints, entry)
				return fmt.Sprintf("status=%d", resp.StatusCode), fmt.Errorf("unexpected status %d", resp.StatusCode)
			}
			summary.Endpoints = append(summary.Endpoints, entry)
			return fmt.Sprintf("status=%d bytes=%d", resp.StatusCode, len(body)), nil
		}); err != nil {
			fail = true
		}
	}

	if readyOK {
		hit("healthz", base+"/healthz", http.StatusOK, "health.txt")
		hit("readyz", base+"/readyz", http.StatusOK, "ready.txt")
		hit("version", base+"/version", http.StatusOK, "version.json")
		hit("metrics", base+"/metrics", http.StatusOK, "metrics.txt")
		hit("openapi", base+"/openapi.json", http.StatusOK, "openapi.json")
		hit("tenants", base+"/v1/tenants", http.StatusOK, "tenants.json")
	} else {
		fail = true
	}

	summary.Passed = !fail
	if fail {
		t.Fail()
	}
}
