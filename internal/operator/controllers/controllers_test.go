package controllers

import (
    "testing"
    v1alpha1 "github.com/vaheed/kubeop/internal/operator/apis/paas/v1alpha1"
    batchv1 "k8s.io/api/batch/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_computeImageRev(t *testing.T) {
    r1 := computeImageRev("nginx:1.25")
    r2 := computeImageRev("nginx:1.26")
    if r1 == r2 || r1 == "" || r2 == "" { t.Fatalf("unexpected revs: %s %s", r1, r2) }
}

// buildHookJobForTest mirrors runHooks job creation for unit testing
func buildHookJobForTest(a *v1alpha1.App, hk v1alpha1.Hook, rev, phase string) *batchv1.Job {
    return buildHookJob(a, hk, rev, phase)
}

func Test_BuildHookJob_Defaults(t *testing.T) {
    a := &v1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "kubeop-acme-web"}}
    hk := v1alpha1.Hook{Image: "alpine:3.20", Args: []string{"/bin/true"}}
    j := buildHookJobForTest(a, hk, "abcd1234", "pre")
    if j == nil { t.Fatalf("nil job") }
    if j.Name != "hook-pre-web-abcd1234" { t.Fatalf("unexpected name: %s", j.Name) }
    if j.Spec.Template.Spec.RestartPolicy != "Never" { t.Fatalf("unexpected restart policy: %s", j.Spec.Template.Spec.RestartPolicy) }
    if len(j.Spec.Template.Spec.Containers) != 1 { t.Fatalf("unexpected containers: %d", len(j.Spec.Template.Spec.Containers)) }
    if j.Spec.Template.Spec.Containers[0].Image != "alpine:3.20" { t.Fatalf("unexpected image: %s", j.Spec.Template.Spec.Containers[0].Image) }
}
