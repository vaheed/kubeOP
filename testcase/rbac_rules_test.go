package testcase

import (
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"

	"kubeop/internal/service"
)

// Ensure default RBAC rules include ReplicaSets for app rollout visibility.
func TestDefaultRBAC_IncludesReplicaSets(t *testing.T) {
	rules := service.DefaultUserNamespaceRoleRules()
	if !ruleCovers(rules, "apps", "replicasets", "get") {
		t.Fatalf("expected replicasets in default RBAC rules under apps group")
	}
}

func TestDefaultRBAC_IncludesEventsAndIngresses(t *testing.T) {
	rules := service.DefaultUserNamespaceRoleRules()
	if !ruleCovers(rules, "", "events", "get") {
		t.Fatalf("expected events in core rbac rules")
	}
	if !ruleCovers(rules, "networking.k8s.io", "ingresses", "get") {
		t.Fatalf("expected ingresses in networking.k8s.io rules")
	}
}

func TestDefaultRBAC_AllowsDeploymentScale(t *testing.T) {
	rules := service.DefaultUserNamespaceRoleRules()
	if !ruleCovers(rules, "apps", "deployments/scale", "patch") {
		t.Fatalf("expected deployments/scale in default RBAC rules")
	}
}

func TestDefaultRBAC_ProvidesWildcardNamespaceAdmin(t *testing.T) {
	rules := service.DefaultUserNamespaceRoleRules()
	found := false
	for _, r := range rules {
		if contains(r.APIGroups, "*") && contains(r.Resources, "*") && contains(r.Verbs, "*") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected wildcard namespace-admin rule in default RBAC")
	}
}

func ruleCovers(rules []rbacv1.PolicyRule, apiGroup, resource, verb string) bool {
	for _, r := range rules {
		if matches(r.APIGroups, apiGroup) && matches(r.Resources, resource) && matches(r.Verbs, verb) {
			return true
		}
	}
	return false
}

func matches(options []string, target string) bool {
	if len(options) == 0 {
		// Empty API group means core APIs; treat it as exact match requirement.
		return target == ""
	}
	for _, candidate := range options {
		if candidate == "*" {
			return true
		}
		if candidate == target {
			return true
		}
		if candidate == "" && target == "" {
			return true
		}
	}
	return false
}

func contains(options []string, target string) bool {
	for _, candidate := range options {
		if candidate == target {
			return true
		}
	}
	return false
}
