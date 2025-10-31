package controllers

import (
    "testing"
    v1alpha1 "github.com/vaheed/kubeop/internal/operator/apis/paas/v1alpha1"
    batchv1 "k8s.io/api/batch/v1"
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

