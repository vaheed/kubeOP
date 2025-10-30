package e2e

import (
    "bytes"
    "encoding/json"
    "io"
    "net/http"
    "os"
    "os/exec"
    "testing"
    "time"
)

func requireTool(t *testing.T, name string) {
    t.Helper()
    if _, err := exec.LookPath(name); err != nil {
        t.Skipf("%s not installed; skipping e2e", name)
    }
}

func Test_EndToEnd_Minimal(t *testing.T) {
    if os.Getenv("KUBEOP_E2E") == "" {
        t.Skip("KUBEOP_E2E not set")
    }
    requireTool(t, "kind")
    requireTool(t, "kubectl")

    // create cluster if not present
    exec.Command("make", "kind-up").Run()
    exec.Command("bash", "-c", "bash e2e/bootstrap.sh").Run()

    // manager should be running via compose; bring it up
    exec.Command("bash", "-c", "docker compose up -d db").Run()
    time.Sleep(3 * time.Second)
    exec.Command("bash", "-c", "docker compose up -d manager").Run()
    time.Sleep(2 * time.Second)

    type obj map[string]any
    httpc := &http.Client{Timeout: 10 * time.Second}
    // create tenant
    b, _ := json.Marshal(obj{"name": "acme"})
    req, _ := http.NewRequest("POST", "http://localhost:18080/v1/tenants", bytes.NewReader(b))
    req.Header.Set("Content-Type", "application/json")
    resp, err := httpc.Do(req)
    if err != nil { t.Fatal(err) }
    out, _ := io.ReadAll(resp.Body); resp.Body.Close()
    if resp.StatusCode != 200 { t.Fatalf("status %d: %s", resp.StatusCode, string(out)) }
    var tenant obj
    if err := json.Unmarshal(out, &tenant); err != nil { t.Fatal(err) }
    tid := tenant["id"].(string)

    // create project
    b, _ = json.Marshal(obj{"tenantID": tid, "name": "web"})
    req, _ = http.NewRequest("POST", "http://localhost:18080/v1/projects", bytes.NewReader(b))
    req.Header.Set("Content-Type", "application/json")
    resp, err = httpc.Do(req)
    if err != nil { t.Fatal(err) }
    out, _ = io.ReadAll(resp.Body); resp.Body.Close()
    if resp.StatusCode != 200 { t.Fatalf("status %d: %s", resp.StatusCode, string(out)) }
    // ingest usage line so invoice subtotal is non-zero
    now := time.Now().UTC().Add(-1 * time.Hour).Truncate(time.Hour)
    payload := []obj{{"ts": now.Format(time.RFC3339), "tenant_id": tid, "cpu_milli": 100, "mem_mib": 200}}
    b, _ = json.Marshal(payload)
    req, _ = http.NewRequest("POST", "http://localhost:18080/v1/usage/ingest", bytes.NewReader(b))
    req.Header.Set("Content-Type", "application/json")
    resp, err = httpc.Do(req)
    if err != nil { t.Fatal(err) }
    io.Copy(io.Discard, resp.Body); resp.Body.Close()

    // usage snapshot
    resp, err = httpc.Get("http://localhost:18080/v1/usage/snapshot")
    if err != nil { t.Fatal(err) }
    out, _ = io.ReadAll(resp.Body); resp.Body.Close()
    if !bytes.Contains(out, []byte("totals")) {
        t.Fatalf("unexpected snapshot: %s", string(out))
    }

    // Create CRDs resources via kubectl and validate Ready
    t.Log("creating Tenant, Project, App, DNSRecord, Certificate CRs")
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
apiVersion: v1
kind: Namespace
metadata:
  name: kubeop-acme-web
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
    cmd := exec.Command("bash", "-lc", "cat <<'YAML' | kubectl apply -f -\n"+yaml+"\nYAML")
    if out, err := cmd.CombinedOutput(); err != nil { t.Fatalf("apply crs: %v: %s", err, string(out)) }
    // wait briefly for reconciliation
    time.Sleep(5 * time.Second)
    // check DNSRecord and Certificate Ready condition
    out, err = exec.Command("bash", "-lc", "kubectl get dnsrecords.paas.kubeop.io web-local -o jsonpath='{.status.ready}'").CombinedOutput()
    if err != nil || !bytes.Contains(out, []byte("true")) {
        t.Fatalf("dnsrecord not ready: %v %s", err, string(out))
    }
    out, err = exec.Command("bash", "-lc", "kubectl get certificates.paas.kubeop.io web-local -o jsonpath='{.status.ready}'").CombinedOutput()
    if err != nil || !bytes.Contains(out, []byte("true")) {
        t.Fatalf("certificate not ready: %v %s", err, string(out))
    }
    // assert app deployment rollout
    out, err = exec.Command("bash", "-lc", "kubectl -n kubeop-acme-web rollout status deploy/app-web --timeout=60s").CombinedOutput()
    if err != nil { t.Fatalf("app rollout: %v %s", err, string(out)) }

    // Inject outages: stop manager and DB then recover
    t.Log("stopping manager (compose)")
    exec.Command("bash", "-lc", "docker compose stop manager").Run()
    time.Sleep(2 * time.Second)
    t.Log("starting manager")
    exec.Command("bash", "-lc", "docker compose start manager").Run()
    // give backlog some time to drain
    time.Sleep(10 * time.Second)
    // stop DB
    t.Log("stopping db")
    exec.Command("bash", "-lc", "docker compose stop db").Run()
    time.Sleep(2 * time.Second)
    t.Log("starting db")
    exec.Command("bash", "-lc", "docker compose start db").Run()
    time.Sleep(10 * time.Second)
    // Verify no drift by re-checking Ready conditions
    out, err = exec.Command("bash", "-lc", "kubectl get dnsrecords.paas.kubeop.io web-local -o jsonpath='{.status.ready}'").CombinedOutput()
    if err != nil || !bytes.Contains(out, []byte("true")) {
        t.Fatalf("dnsrecord not ready after recovery: %v %s", err, string(out))
    }
    out, err = exec.Command("bash", "-lc", "kubectl get certificates.paas.kubeop.io web-local -o jsonpath='{.status.ready}'").CombinedOutput()
    if err != nil || !bytes.Contains(out, []byte("true")) {
        t.Fatalf("certificate not ready after recovery: %v %s", err, string(out))
    }

    // Collect artifacts
    _ = os.MkdirAll("artifacts", 0o755)
    exec.Command("bash", "-lc", "kubectl -n kubeop-system logs deploy/kubeop-operator --tail=-1 > artifacts/operator.log 2>&1").Run()
    exec.Command("bash", "-lc", "docker compose logs manager > artifacts/manager.log 2>&1").Run()
    exec.Command("bash", "-lc", "kubectl get events -A --sort-by=.lastTimestamp > artifacts/events.txt 2>&1").Run()
    exec.Command("bash", "-lc", "kubectl get all -A -o wide > artifacts/resources.txt 2>&1").Run()
}
