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

func Test_EndToEnd_Minimal(t *testing.T) {
	if os.Getenv("KUBEOP_E2E") == "" {
		t.Skip("KUBEOP_E2E not set")
	}
	requireTool(t, "kind")
	requireTool(t, "kubectl")
	requireTool(t, "docker")
	requireTool(t, "helm")

	recorder := NewResultsRecorder(t, "kind")

	recorder.MustStep("ensure kind cluster", func() (string, error) {
		return runCommand(t, "make", []string{"kind-up"})
	})
	recorder.MustStep("bootstrap kind assets", func() (string, error) {
		return runCommand(t, "bash", []string{"-c", "bash e2e/bootstrap.sh"})
	})

	artifacts := os.Getenv("ARTIFACTS_DIR")
	if artifacts == "" {
		artifacts = "artifacts"
	}
	if err := os.MkdirAll(artifacts, 0o755); err != nil {
		t.Fatalf("artifacts dir: %v", err)
	}
	t.Cleanup(func() {
		_, _ = runCommand(t, "bash", []string{"-lc", "docker compose ps > " + filepath.Join(artifacts, "compose-ps.txt") + " 2>&1"})
		_, _ = runCommand(t, "bash", []string{"-lc", "docker compose logs manager > " + filepath.Join(artifacts, "manager.log") + " 2>&1"})
		_, _ = runCommand(t, "bash", []string{"-lc", "docker compose logs db > " + filepath.Join(artifacts, "db.log") + " 2>&1"})
	})

	recorder.MustStep("start database", func() (string, error) {
		return runCommand(t, "bash", []string{"-c", "docker compose up -d db"})
	})
	recorder.MustStep("start manager", func() (string, error) {
		return runCommand(t, "bash", []string{"-c", "KUBEOP_AGGREGATOR=true docker compose up -d manager"})
	})

	mgrURL := "http://localhost:18080"
	recorder.MustStep("wait for manager readiness", func() (string, error) {
		deadline := time.Now().Add(90 * time.Second)
		attempts := 0
		for {
			attempts++
			if time.Now().After(deadline) {
				return fmt.Sprintf("attempts=%d", attempts), fmt.Errorf("manager not ready within 90s")
			}
			resp, err := http.Get(mgrURL + "/readyz")
			if err == nil {
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return fmt.Sprintf("attempts=%d", attempts), nil
				}
			}
			time.Sleep(2 * time.Second)
		}
	})

	type obj map[string]any
	httpc := &http.Client{Timeout: 10 * time.Second}

	var tenantID string
	recorder.MustStep("create tenant", func() (string, error) {
		b, _ := json.Marshal(obj{"name": "acme"})
		req, _ := http.NewRequest(http.MethodPost, mgrURL+"/v1/tenants", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		resp, err := httpc.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return string(body), fmt.Errorf("status %d", resp.StatusCode)
		}
		var tenant obj
		if err := json.Unmarshal(body, &tenant); err != nil {
			return string(body), err
		}
		tenantID, _ = tenant["id"].(string)
		if tenantID == "" {
			return string(body), fmt.Errorf("missing tenant id")
		}
		return fmt.Sprintf("tenant_id=%s", tenantID), nil
	})

	recorder.MustStep("create project", func() (string, error) {
		b, _ := json.Marshal(obj{"tenantID": tenantID, "name": "web"})
		req, _ := http.NewRequest(http.MethodPost, mgrURL+"/v1/projects", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		resp, err := httpc.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return string(body), fmt.Errorf("status %d", resp.StatusCode)
		}
		return fmt.Sprintf("project bytes=%d", len(body)), nil
	})

	recorder.MustStep("ingest usage", func() (string, error) {
		now := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Hour)
		payload := []obj{{"ts": now.Format(time.RFC3339), "tenant_id": tenantID, "cpu_milli": 100, "mem_mib": 200}}
		b, _ := json.Marshal(payload)
		req, _ := http.NewRequest(http.MethodPost, mgrURL+"/v1/usage/ingest", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		resp, err := httpc.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return string(body), fmt.Errorf("status %d", resp.StatusCode)
		}
		return fmt.Sprintf("payload=%s", string(b)), nil
	})

	recorder.MustStep("usage snapshot", func() (string, error) {
		resp, err := httpc.Get(mgrURL + "/v1/usage/snapshot")
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if !bytes.Contains(body, []byte("totals")) {
			return string(body), fmt.Errorf("missing totals key")
		}
		return fmt.Sprintf("bytes=%d", len(body)), nil
	})

	yaml := `apiVersion: paas.kubeop.io/v1alpha1
kind: Tenant
metadata:
  name: acme
spec:
  name: acme
---
apiVersion: paas.kubeop.io/v1alpha1
kind: Project
metadata:
  name: web
spec:
  tenantRef: acme
  name: web
---
apiVersion: paas.kubeop.io/v1alpha1
kind: App
metadata:
  name: web
  namespace: kubeop-acme-web
spec:
  type: Image
  image: nginx:1.25
  host: web.local
---
apiVersion: paas.kubeop.io/v1alpha1
kind: DNSRecord
metadata:
  name: web-local
spec:
  host: web.local
  target: web.kubeop-acme-web.svc.cluster.local
---
apiVersion: paas.kubeop.io/v1alpha1
kind: Certificate
metadata:
  name: web-local
spec:
  host: web.local
  dnsRecordRef: web-local
`

	recorder.MustStep("apply sample resources", func() (string, error) {
		return runCommand(t, "kubectl", []string{"apply", "-f", "-"}, WithStdin(bytes.NewBufferString(yaml)))
	})

	recorder.MustStep("wait for operator deployment", func() (string, error) {
		start := time.Now()
		for time.Since(start) < 90*time.Second {
			out, err := runCommand(t, "kubectl", []string{"-n", "kubeop-system", "get", "deploy/kubeop-operator", "-o", "jsonpath={.status.availableReplicas}"})
			if err == nil && bytes.Contains([]byte(out), []byte("1")) {
				return fmt.Sprintf("ready_in=%s", time.Since(start)), nil
			}
			time.Sleep(3 * time.Second)
		}
		return "", fmt.Errorf("operator not ready within 90s")
	})

	time.Sleep(5 * time.Second)

	recorder.MustStep("dnsrecord ready", func() (string, error) {
		for i := 0; i < 20; i++ {
			out, err := runCommand(t, "kubectl", []string{"get", "dnsrecords.paas.kubeop.io", "web-local", "-o", "jsonpath={.status.ready}"})
			if err == nil && bytes.Contains([]byte(out), []byte("true")) {
				return fmt.Sprintf("iterations=%d", i+1), nil
			}
			time.Sleep(3 * time.Second)
		}
		return "", fmt.Errorf("dnsrecord not ready")
	})

	recorder.MustStep("certificate ready", func() (string, error) {
		for i := 0; i < 20; i++ {
			out, err := runCommand(t, "kubectl", []string{"get", "certificates.paas.kubeop.io", "web-local", "-o", "jsonpath={.status.ready}"})
			if err == nil && bytes.Contains([]byte(out), []byte("true")) {
				return fmt.Sprintf("iterations=%d", i+1), nil
			}
			time.Sleep(3 * time.Second)
		}
		return "", fmt.Errorf("certificate not ready")
	})

	recorder.MustStep("app rollout", func() (string, error) {
		return runCommand(t, "kubectl", []string{"-n", "kubeop-acme-web", "rollout", "status", "deploy/app-web", "--timeout=60s"})
	})

	recorder.MustStep("restart manager", func() (string, error) {
		if _, err := runCommand(t, "bash", []string{"-lc", "docker compose stop manager"}); err != nil {
			return "", err
		}
		time.Sleep(2 * time.Second)
		if _, err := runCommand(t, "bash", []string{"-lc", "docker compose start manager"}); err != nil {
			return "", err
		}
		start := time.Now()
		for time.Since(start) < 2*time.Minute {
			out, err := runCommand(t, "kubectl", []string{"-n", "kubeop-acme-web", "get", "deploy/app-web", "-o", "jsonpath={.status.availableReplicas}"})
			if err == nil && bytes.Contains([]byte(out), []byte("1")) {
				return fmt.Sprintf("drained_in=%s", time.Since(start)), nil
			}
			time.Sleep(5 * time.Second)
		}
		return "", fmt.Errorf("backlog did not drain within 2 minutes")
	})

	recorder.MustStep("restart database", func() (string, error) {
		if _, err := runCommand(t, "bash", []string{"-lc", "docker compose stop db"}); err != nil {
			return "", err
		}
		time.Sleep(2 * time.Second)
		if _, err := runCommand(t, "bash", []string{"-lc", "docker compose start db"}); err != nil {
			return "", err
		}
		time.Sleep(10 * time.Second)
		return "db restarted", nil
	})

	recorder.MustStep("post-restart dnsready", func() (string, error) {
		out, err := runCommand(t, "kubectl", []string{"get", "dnsrecords.paas.kubeop.io", "web-local", "-o", "jsonpath={.status.ready}"})
		if err != nil || !bytes.Contains([]byte(out), []byte("true")) {
			return out, fmt.Errorf("dnsrecord not ready after recovery")
		}
		return "ready=true", nil
	})

	recorder.MustStep("post-restart certready", func() (string, error) {
		out, err := runCommand(t, "kubectl", []string{"get", "certificates.paas.kubeop.io", "web-local", "-o", "jsonpath={.status.ready}"})
		if err != nil || !bytes.Contains([]byte(out), []byte("true")) {
			return out, fmt.Errorf("certificate not ready after recovery")
		}
		return "ready=true", nil
	})

	recorder.MustStep("collect artifacts", func() (string, error) {
		commands := []struct {
			name string
			cmd  []string
		}{
			{"operator logs", []string{"-lc", "kubectl -n kubeop-system logs deploy/kubeop-operator --tail=-1 > " + filepath.Join(artifacts, "operator.log") + " 2>&1"}},
			{"manager logs", []string{"-lc", "docker compose logs manager > " + filepath.Join(artifacts, "manager.log") + " 2>&1"}},
			{"events", []string{"-lc", "kubectl get events -A --sort-by=.lastTimestamp > " + filepath.Join(artifacts, "events.txt") + " 2>&1"}},
			{"resources", []string{"-lc", "kubectl get all -A -o wide > " + filepath.Join(artifacts, "resources.txt") + " 2>&1"}},
			{"metrics", []string{"-lc", "kubectl run curl-metrics --rm -i --restart=Never --image=curlimages/curl:8.7.1 -- curl -s kubeop-operator-metrics.kubeop-system.svc.cluster.local:8081/metrics | head -n 200 > " + filepath.Join(artifacts, "operator-metrics.txt") + " 2>&1"}},
			{"db snapshot", []string{"-lc", "command -v pg_dump >/dev/null 2>&1 && pg_dump -h localhost -U kubeop -d kubeop > " + filepath.Join(artifacts, "db.sql") + " || true"}},
		}
		for _, c := range commands {
			if _, err := runCommand(t, "bash", c.cmd); err != nil {
				return c.name, err
			}
		}
		return fmt.Sprintf("artifacts=%s", artifacts), nil
	})
}
