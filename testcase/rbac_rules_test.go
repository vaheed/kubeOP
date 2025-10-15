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

func TestDefaultRBAC_AllowsWorkloadScaling(t *testing.T) {
	rules := service.DefaultUserNamespaceRoleRules()
	if !ruleCovers(rules, "apps", "deployments/scale", "patch") {
		t.Fatalf("expected deployments/scale in default RBAC rules")
	}
	if !ruleCovers(rules, "apps", "statefulsets/scale", "patch") {
		t.Fatalf("expected statefulsets/scale in default RBAC rules")
	}
}

func TestDefaultRBAC_DoesNotProvideNamespaceWildcards(t *testing.T) {
	rules := service.DefaultUserNamespaceRoleRules()
	for _, r := range rules {
		if contains(r.APIGroups, "*") {
			t.Fatalf("unexpected API group wildcard in rule: %+v", r)
		}
		if contains(r.Resources, "*") {
			t.Fatalf("unexpected resource wildcard in rule: %+v", r)
		}
		if contains(r.Verbs, "*") {
			t.Fatalf("unexpected verb wildcard in rule: %+v", r)
		}
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
