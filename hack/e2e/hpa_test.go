package e2e

import (
    "bytes"
    "fmt"
    "os/exec"
    "path/filepath"
    "runtime"
    "os"
    "testing"
    "time"
)

func chartPath() string {
    // this file lives in hack/e2e; chart lives at ../charts/kubeop-operator
    _, file, _, _ := runtime.Caller(0)
    return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "charts", "kubeop-operator"))
}

// Test_HPA_ScalesOperator generates load by creating many Apps and enabling reconcile spin.
// It asserts the HPA scales the operator above 1 replica, then optionally returns to 1.
func Test_HPA_ScalesOperator(t *testing.T) {
    if os.Getenv("KUBEOP_E2E") == "" {
        t.Skip("KUBEOP_E2E not set")
    }
    artifacts := os.Getenv("ARTIFACTS_DIR")
    if artifacts == "" { artifacts = "artifacts" }
    _ = os.MkdirAll(filepath.Join(artifacts, "hpa"), 0o755)

    // Reconfigure operator HPA for aggressive scaling and enable spin
    cmd := exec.Command("bash", "-lc", fmt.Sprintf("helm upgrade kubeop-operator %s -n kubeop-system --reuse-values --set hpa.enabled=true --set hpa.minReplicas=1 --set hpa.maxReplicas=4 --set hpa.targetCPUUtilizationPercentage=10 --set resources.requests.cpu=10m --set resources.limits.cpu=200m --set loadTest.reconcileSpinMs=50", chartPath()))
    if out, err := cmd.CombinedOutput(); err != nil {
        t.Fatalf("helm upgrade: %v: %s", err, string(out))
    }
    // Wait for rollout
    if out, err := exec.Command("bash", "-lc", "kubectl -n kubeop-system rollout status deploy/kubeop-operator --timeout=180s").CombinedOutput(); err != nil {
        t.Fatalf("operator rollout: %v: %s", err, string(out))
    }

    // Create many Apps to trigger reconciles
    var buf bytes.Buffer
    buf.WriteString("apiVersion: paas.kubeop.io/v1alpha1\nkind: Tenant\nmetadata:\n  name: loadtenant\nspec:\n  name: loadtenant\n---\n")
    buf.WriteString("apiVersion: paas.kubeop.io/v1alpha1\nkind: Project\nmetadata:\n  name: loadproj\nspec:\n  tenantRef: loadtenant\n  name: loadproj\n---\n")
    // Namespace will be created by operator
    for i := 0; i < 50; i++ {
        buf.WriteString(fmt.Sprintf("apiVersion: paas.kubeop.io/v1alpha1\nkind: App\nmetadata:\n  name: load-%d\n  namespace: kubeop-loadtenant-loadproj\nspec:\n  type: Image\n  image: nginx:1.25\n\n---\n", i))
    }
    apply := exec.Command("bash", "-lc", "cat <<'YAML' | kubectl apply -f -\n"+buf.String()+"\nYAML")
    if out, err := apply.CombinedOutput(); err != nil {
        t.Fatalf("apply load apps: %v: %s", err, string(out))
    }

    // Wait for HPA to scale above 1 replica
    scaled := false
    deadline := time.Now().Add(3 * time.Minute)
    for time.Now().Before(deadline) {
        out, _ := exec.Command("bash", "-lc", "kubectl -n kubeop-system get hpa kubeop-operator -o jsonpath='{.status.desiredReplicas}'").CombinedOutput()
        if bytes.Contains(out, []byte("2")) || bytes.Contains(out, []byte("3")) || bytes.Contains(out, []byte("4")) {
            scaled = true
            break
        }
        time.Sleep(5 * time.Second)
    }
    if !scaled {
        t.Fatalf("HPA did not scale operator above 1 replica")
    }

    // Dump HPA status and operator deployment to artifacts
    exec.Command("bash", "-lc", "kubectl -n kubeop-system get hpa kubeop-operator -o wide > "+filepath.Join(artifacts, "hpa", "hpa.txt")+" 2>&1").Run()
    exec.Command("bash", "-lc", "kubectl -n kubeop-system get deploy kubeop-operator -o yaml > "+filepath.Join(artifacts, "hpa", "operator-deploy.yaml")+" 2>&1").Run()

    // Begin scale-down: disable spin and delete load Apps
    if out, err := exec.Command("bash", "-lc", fmt.Sprintf("helm upgrade kubeop-operator %s -n kubeop-system --reuse-values --set loadTest.reconcileSpinMs=0", chartPath())).CombinedOutput(); err != nil {
        t.Fatalf("helm disable spin: %v: %s", err, string(out))
    }
    // Wait for rollout
    if out, err := exec.Command("bash", "-lc", "kubectl -n kubeop-system rollout status deploy/kubeop-operator --timeout=180s").CombinedOutput(); err != nil {
        t.Fatalf("operator rollout (disable spin): %v: %s", err, string(out))
    }
    // Delete Apps to remove work
    _ = exec.Command("bash", "-lc", "kubectl -n kubeop-loadtenant-loadproj delete apps.paas.kubeop.io --all --ignore-not-found").Run()

    // Reduce downscale stabilization via helm (chart supports behavior)
    if out, err := exec.Command("bash", "-lc", fmt.Sprintf("helm upgrade kubeop-operator %s -n kubeop-system --reuse-values --set hpa.behavior.scaleDown.stabilizationWindowSeconds=0", chartPath())).CombinedOutput(); err != nil {
        t.Fatalf("helm set behavior: %v: %s", err, string(out))
    }

    // Poll HPA desiredReplicas until it returns to 1 within 6 minutes
    timeline := filepath.Join(artifacts, "hpa", "scale_timeline.txt")
    _ = os.WriteFile(timeline, []byte("timestamp,current/desired\n"), 0o644)
    scaledDown := false
    downDeadline := time.Now().Add(6 * time.Minute)
    for time.Now().Before(downDeadline) {
        out, _ := exec.Command("bash", "-lc", "kubectl -n kubeop-system get hpa kubeop-operator -o jsonpath='{.status.currentReplicas}/{.status.desiredReplicas}'").CombinedOutput()
        now := time.Now().UTC().Format(time.RFC3339)
        f, _ := os.OpenFile(timeline, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
        fmt.Fprintf(f, "%s %s\n", now, string(out))
        f.Close()
        if bytes.HasSuffix(bytes.TrimSpace(out), []byte("1/1")) || bytes.Equal(bytes.TrimSpace(out), []byte("1/1")) {
            scaledDown = true
            break
        }
        time.Sleep(10 * time.Second)
    }
    if !scaledDown {
        t.Fatalf("HPA did not scale down to 1 replica within window")
    }
}
