package testcase

import (
    "testing"
    "kubeop/internal/service"
)

// Ensure default RBAC rules include ReplicaSets for app rollout visibility.
func TestDefaultRBAC_IncludesReplicaSets(t *testing.T) {
    rules := service.DefaultUserNamespaceRoleRules()
    found := false
    for _, r := range rules {
        if r.APIGroups != nil && len(r.APIGroups) > 0 && r.APIGroups[0] == "apps" {
            for _, res := range r.Resources {
                if res == "replicasets" { found = true; break }
            }
        }
    }
    if !found {
        t.Fatalf("expected replicasets in default RBAC rules under apps group")
    }
}

