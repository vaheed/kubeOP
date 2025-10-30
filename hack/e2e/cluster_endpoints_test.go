package e2e

import (
	"context"
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

// Test_ClusterEndpoints validates operator in-cluster endpoints via port-forward
// It hits: /healthz (8082), /readyz (8082), /version (8083), /metrics (8081)
func Test_ClusterEndpoints(t *testing.T) {
	if os.Getenv("KUBEOP_E2E") == "" {
		t.Skip("KUBEOP_E2E not set")
	}
	requireTool(t, "kubectl")

	artifacts := os.Getenv("ARTIFACTS_DIR")
	if artifacts == "" {
		artifacts = "artifacts"
	}
	outDir := filepath.Join(artifacts, "cluster")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatalf("create cluster dir: %v", err)
	}

	recorder := NewResultsRecorder(t, "cluster")

	recorder.MustStep("wait operator ready", func() (string, error) {
		deadline := time.Now().Add(2 * time.Minute)
		attempts := 0
		for {
			attempts++
			if time.Now().After(deadline) {
				return fmt.Sprintf("attempts=%d", attempts), fmt.Errorf("operator not ready within 2m")
			}
			out, err := runCommand(t, "kubectl", []string{"-n", "kubeop-system", "get", "deploy/kubeop-operator", "-o", "jsonpath={.status.availableReplicas}"})
			if err == nil && out == "1" {
				return fmt.Sprintf("attempts=%d", attempts), nil
			}
			time.Sleep(3 * time.Second)
		}
	})

	ctx, cancel := context.WithCancel(context.Background())
	var pfErr error
	recorder.MustStep("start port-forward", func() (string, error) {
		cmd := execCommandContext(ctx, "kubectl", "-n", "kubeop-system", "port-forward", "deploy/kubeop-operator", "18081:8081", "18082:8082", "18083:8083")
		if err := cmd.Start(); err != nil {
			cancel()
			return "", err
		}
		go func() {
			pfErr = cmd.Wait()
		}()
		time.Sleep(2 * time.Second)
		return "ports=18081,18082,18083", nil
	})

	t.Cleanup(func() {
		cancel()
		time.Sleep(500 * time.Millisecond)
		if pfErr != nil && pfErr.Error() != "signal: killed" {
			t.Logf("port-forward exited: %v", pfErr)
		}
	})

	httpc := &http.Client{Timeout: 5 * time.Second}

	check := func(name, url string, want int, file string, validate func([]byte) error) {
		step := "cluster:" + name
		if err := recorder.Step(step, func() (string, error) {
			resp, err := httpc.Get(url)
			if err != nil {
				_ = os.WriteFile(filepath.Join(outDir, file), []byte(err.Error()), 0o644)
				return "", err
			}
			defer resp.Body.Close()
			body, _ := io.ReadAll(resp.Body)
			_ = os.WriteFile(filepath.Join(outDir, file), append([]byte(resp.Status+"\n"), body...), 0o644)
			if resp.StatusCode != want {
				return fmt.Sprintf("status=%d", resp.StatusCode), fmt.Errorf("unexpected status %d", resp.StatusCode)
			}
			if validate != nil {
				if err := validate(body); err != nil {
					return "", err
				}
			}
			return fmt.Sprintf("status=%d bytes=%d", resp.StatusCode, len(body)), nil
		}); err != nil {
			t.Fatalf("%s: %v", step, err)
		}
	}

	check("healthz", "http://127.0.0.1:18082/healthz", http.StatusOK, "health.txt", nil)
	check("readyz", "http://127.0.0.1:18082/readyz", http.StatusOK, "ready.txt", nil)
	check("version", "http://127.0.0.1:18083/version", http.StatusOK, "version.json", func(body []byte) error {
		var v map[string]any
		if err := json.Unmarshal(body, &v); err != nil {
			return err
		}
		if svc, _ := v["service"].(string); svc != "operator" {
			return fmt.Errorf("unexpected service: %v", svc)
		}
		return nil
	})
	check("metrics", "http://127.0.0.1:18081/metrics", http.StatusOK, "metrics.txt", nil)
}

func execCommandContext(ctx context.Context, name string, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	return cmd
}
