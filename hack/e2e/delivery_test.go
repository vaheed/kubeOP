package e2e

import (
    "bytes"
    "os/exec"
    "testing"
    "time"
)

func Test_Delivery_Image_Update_Rollout(t *testing.T) {
    if skipE2E(t) { return }
    // ensure namespace exists
    ns := "kubeop-acme-web"
    _, _ = exec.Command("bash", "-lc", "kubectl get ns "+ns+" >/dev/null 2>&1 || kubectl create ns "+ns).CombinedOutput()

    // create App with nginx:1.25
    app := `apiVersion: paas.kubeop.io/v1alpha1
kind: App
metadata:
  name: roll
  namespace: `+ns+`
spec:
  type: Image
  image: nginx:1.25
`
    if out, err := exec.Command("bash", "-lc", "cat <<'YAML' | kubectl apply -f -\n"+app+"\nYAML").CombinedOutput(); err != nil {
        t.Fatalf("apply app: %v: %s", err, string(out))
    }
    time.Sleep(3 * time.Second)
    // capture revision
    out, _ := exec.Command("bash", "-lc", "kubectl -n "+ns+" get apps.paas.kubeop.io roll -o jsonpath='{.status.revision}'").CombinedOutput()
    rev1 := string(bytes.TrimSpace(out))
    if rev1 == "" { t.Fatalf("missing initial revision") }
    // update image
    if out, err := exec.Command("bash", "-lc", "kubectl -n "+ns+" patch app roll --type=merge -p '{"spec":{"image":"nginx:1.26"}}'").CombinedOutput(); err != nil {
        t.Fatalf("patch app: %v: %s", err, string(out))
    }
    // wait for revision change and template image update
    var rev2 string
    for i := 0; i < 30; i++ {
        out, _ = exec.Command("bash", "-lc", "kubectl -n "+ns+" get apps.paas.kubeop.io roll -o jsonpath='{.status.revision}'").CombinedOutput()
        rev2 = string(bytes.TrimSpace(out))
        if rev2 != "" && rev2 != rev1 {
            // verify Deployment template image updated
            out, _ = exec.Command("bash", "-lc", "kubectl -n "+ns+" get deploy app-roll -o jsonpath='{.spec.template.spec.containers[0].image}'").CombinedOutput()
            img := string(bytes.TrimSpace(out))
            if bytes.Contains([]byte(img), []byte("nginx:1.26")) { return }
        }
        time.Sleep(2 * time.Second)
    }
    t.Fatalf("revision/template image did not update: rev1=%s rev2=%s", rev1, rev2)
}

func skipE2E(t *testing.T) bool {
    if v := getEnv("KUBEOP_E2E"); v == "" { t.Skip("KUBEOP_E2E not set"); return true }
    return false
}

func getEnv(k string) string { b, _ := exec.Command("bash", "-lc", "printf %s $"+k).CombinedOutput(); return string(bytes.TrimSpace(b)) }
