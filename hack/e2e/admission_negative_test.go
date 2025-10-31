package e2e

import (
    "bytes"
    "os"
    "os/exec"
    "testing"
)

// Deny App with disallowed image registry; ensure mutating adds label on allowed app
func Test_Admission_Denies_Disallowed_Image(t *testing.T) {
    if os.Getenv("KUBEOP_E2E") == "" { t.Skip("KUBEOP_E2E not set") }
    // Ensure namespace exists with required labels (operator may not have created it yet)
    _ = exec.Command("bash", "-lc", "kubectl get ns kubeop-acme-web >/dev/null 2>&1 || cat <<'YAML' | kubectl apply -f -\napiVersion: v1\nkind: Namespace\nmetadata:\n  name: kubeop-acme-web\n  labels:\n    app.kubeop.io/tenant: acme\n    app.kubeop.io/project: web\nYAML").Run()
    // Apply disallowed app in kubeop-acme-web
    yaml := `apiVersion: paas.kubeop.io/v1alpha1
kind: App
metadata:
  name: bad
  namespace: kubeop-acme-web
spec:
  type: Image
  image: evil.io/forbidden:latest
`
    cmd := exec.Command("bash", "-lc", "cat <<'YAML' | kubectl apply -f -\n"+yaml+"\nYAML")
    out, err := cmd.CombinedOutput()
    if err == nil {
        t.Fatalf("expected admission denial, got success: %s", string(out))
    }

    // Allowed app should succeed and be mutated (label present)
    yaml2 := `apiVersion: paas.kubeop.io/v1alpha1
kind: App
metadata:
  name: good
  namespace: kubeop-acme-web
spec:
  type: Image
  image: docker.io/library/nginx:1.25
`
    cmd = exec.Command("bash", "-lc", "cat <<'YAML' | kubectl apply -f -\n"+yaml2+"\nYAML")
    out, err = cmd.CombinedOutput()
    if err != nil { t.Fatalf("apply allowed app: %v: %s", err, string(out)) }
    // fetch labels
    out, err = exec.Command("bash", "-lc", "kubectl -n kubeop-acme-web get apps.paas.kubeop.io good -o jsonpath='{.metadata.labels.app\\.kubeop\\.io/managed-by}'").CombinedOutput()
    if err != nil || !bytes.Contains(out, []byte("kubeop-admission")) {
        t.Fatalf("mutating label not set: %v %s", err, string(out))
    }
}
