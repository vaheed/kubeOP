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

func TestDefaultRBAC_IncludesEventsAndIngresses(t *testing.T) {
    rules := service.DefaultUserNamespaceRoleRules()
    haveEvents := false
    haveIngress := false
    for _, r := range rules {
        if len(r.APIGroups) == 0 || r.APIGroups[0] == "" {
            for _, res := range r.Resources { if res == "events" { haveEvents = true; break } }
        }
        if len(r.APIGroups) > 0 && r.APIGroups[0] == "networking.k8s.io" {
            for _, res := range r.Resources { if res == "ingresses" { haveIngress = true; break } }
        }
    }
    if !haveEvents { t.Fatalf("expected events in core rbac rules") }
    if !haveIngress { t.Fatalf("expected ingresses in networking.k8s.io rules") }
}
