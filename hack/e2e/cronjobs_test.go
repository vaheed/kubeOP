package e2e

import (
    "bytes"
    "encoding/base64"
    "encoding/json"
    "io"
    "net/http"
    "os"
    "os/exec"
    "testing"
    "time"
)

// CronJobs API happy path: create tenant+project, create cronjob, list, run, delete
func Test_CronJobs_Flow(t *testing.T) {
    if os.Getenv("KUBEOP_E2E") == "" { t.Skip("KUBEOP_E2E not set") }
    mgr := "http://localhost:18080"
    httpc := &http.Client{Timeout: 15 * time.Second}

    // Register cluster and capture ID so manager can use stored kubeconfig
    // (avoids relying on container KUBECONFIG env when running via compose)
    out, err := exec.Command("bash", "-lc", "kubectl config view --raw").CombinedOutput()
    if err != nil || len(out) == 0 { t.Skipf("no kubeconfig: %v", err) }
    b64 := base64.StdEncoding.EncodeToString(out)
    cb, _ := json.Marshal(map[string]any{"name": "kind-kubeop", "kubeconfig": b64})
    resp, err := httpc.Post(mgr+"/v1/clusters", "application/json", bytes.NewReader(cb))
    if err != nil { t.Fatal(err) }
    raw, _ := io.ReadAll(resp.Body); resp.Body.Close()
    if resp.StatusCode != 200 { t.Fatalf("cluster create: %d %s", resp.StatusCode, string(raw)) }
    var c map[string]any; _ = json.Unmarshal(raw, &c)
    cid := c["id"].(string)

    // Create tenant
    b, _ := json.Marshal(map[string]any{"name": "cj-acme", "clusterID": cid})
    resp, err := httpc.Post(mgr+"/v1/tenants", "application/json", bytes.NewReader(b))
    if err != nil { t.Fatal(err) }
    out, _ = io.ReadAll(resp.Body); resp.Body.Close()
    if resp.StatusCode != 200 { t.Fatalf("tenant: %d %s", resp.StatusCode, string(out)) }
    var ten map[string]any; _ = json.Unmarshal(out, &ten)
    tid := ten["id"].(string)
    // Create project
    b, _ = json.Marshal(map[string]any{"tenantID": tid, "name": "jobs"})
    resp, err = httpc.Post(mgr+"/v1/projects", "application/json", bytes.NewReader(b))
    if err != nil { t.Fatal(err) }
    out, _ = io.ReadAll(resp.Body); resp.Body.Close()
    if resp.StatusCode != 200 { t.Fatalf("project: %d %s", resp.StatusCode, string(out)) }
    var pr map[string]any; _ = json.Unmarshal(out, &pr)
    pid := pr["id"].(string)

    // Create CronJob (runs busybox)
    body := map[string]any{"projectID": pid, "name": "echo", "schedule": "*/5 * * * *", "image": "busybox", "command": []string{"/bin/sh", "-c"}, "args": []string{"echo hi"}}
    b, _ = json.Marshal(body)
    resp, err = httpc.Post(mgr+"/v1/cronjobs", "application/json", bytes.NewReader(b))
    if err != nil { t.Fatal(err) }
    out, _ = io.ReadAll(resp.Body); resp.Body.Close()
    if resp.StatusCode != 200 { t.Fatalf("cronjob create: %d %s", resp.StatusCode, string(out)) }

    // List
    resp, err = httpc.Get(mgr+"/v1/cronjobs?projectID="+pid)
    if err != nil { t.Fatal(err) }
    out, _ = io.ReadAll(resp.Body); resp.Body.Close()
    if !bytes.Contains(out, []byte("echo")) { t.Fatalf("list missing cronjob: %s", string(out)) }

    // Trigger run
    req, _ := http.NewRequest("POST", mgr+"/v1/cronjobs/"+pid+"/echo/run", nil)
    resp, err = httpc.Do(req)
    if err != nil { t.Fatal(err) }
    io.Copy(io.Discard, resp.Body); resp.Body.Close()
}

// Cluster ready endpoint: register current kubeconfig and wait ready
func Test_Cluster_Ready_Endpoint(t *testing.T) {
    if os.Getenv("KUBEOP_E2E") == "" { t.Skip("KUBEOP_E2E not set") }
    mgr := "http://localhost:18080"
    httpc := &http.Client{Timeout: 20 * time.Second}
    // Get current kubeconfig
    out, err := exec.Command("bash", "-lc", "kubectl config view --raw").CombinedOutput()
    if err != nil || len(out) == 0 { t.Skipf("no kubeconfig: %v", err) }
    b64 := base64.StdEncoding.EncodeToString(out)
    // Register cluster with auto bootstrap (idempotent if already installed)
    body := map[string]any{"name": "kind-kubeop", "kubeconfig": b64, "autoBootstrap": true, "installAdmission": true, "withMocks": true}
    b, _ := json.Marshal(body)
    resp, err := httpc.Post(mgr+"/v1/clusters", "application/json", bytes.NewReader(b))
    if err != nil { t.Fatal(err) }
    out, _ = io.ReadAll(resp.Body); resp.Body.Close()
    if resp.StatusCode != 200 { t.Fatalf("cluster create: %d %s", resp.StatusCode, string(out)) }
    var c map[string]any; _ = json.Unmarshal(out, &c)
    id := c["id"].(string)

    // Poll ready endpoint (it returns 200 when both operator+admission ready and caBundle set)
    deadline := time.Now().Add(3 * time.Minute)
    for time.Now().Before(deadline) {
        r, err := httpc.Get(mgr+"/v1/clusters/"+id+"/ready")
        if err == nil && r.StatusCode == 200 { io.Copy(io.Discard, r.Body); r.Body.Close(); return }
        if r != nil { io.Copy(io.Discard, r.Body); r.Body.Close() }
        time.Sleep(5 * time.Second)
    }
    t.Fatalf("cluster did not become ready in time")
}
