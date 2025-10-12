package testcase

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	"kubeop/internal/config"
	"kubeop/internal/service"
)

func TestBuildTenantNetworkPolicies(t *testing.T) {
	cfg := &config.Config{
		DNSNamespaceLabelKey:       "kubernetes.io/metadata.name",
		DNSNamespaceLabelValue:     "kube-system",
		DNSPodLabelKey:             "k8s-app",
		DNSPodLabelValue:           "kube-dns",
		IngressNamespaceLabelKey:   "ingress",
		IngressNamespaceLabelValue: "enabled",
	}

	policies := service.BuildTenantNetworkPolicies(cfg, "tenant-a")
	if len(policies) != 3 {
		t.Fatalf("expected 3 network policies, got %d", len(policies))
	}
	names := map[string]bool{}
	for _, np := range policies {
		names[np.Name] = true
		if np.Namespace != "tenant-a" {
			t.Fatalf("unexpected namespace: %s", np.Namespace)
		}
	}
	for _, want := range []string{"default-deny", "allow-dns", "allow-from-ingress"} {
		if !names[want] {
			t.Fatalf("missing policy %q", want)
		}
	}

	dns := policies[1]
	if len(dns.Spec.Egress) != 1 {
		t.Fatalf("expected dns egress rule")
	}
	if dns.Spec.Egress[0].To[0].NamespaceSelector.MatchLabels[cfg.DNSNamespaceLabelKey] != cfg.DNSNamespaceLabelValue {
		t.Fatalf("dns namespace selector mismatch")
	}
}

func TestBuildNamespaceRBAC(t *testing.T) {
	rules := []rbacv1.PolicyRule{{APIGroups: []string{""}, Resources: []string{"pods"}, Verbs: []string{"get"}}}
	sa, role, rb := service.BuildNamespaceRBAC("tenant-a", "svc", "role", "bind", rules)

	if sa.Name != "svc" || sa.Namespace != "tenant-a" {
		t.Fatalf("unexpected service account: %+v", sa)
	}
	if role.Name != "role" || role.Namespace != "tenant-a" {
		t.Fatalf("unexpected role: %+v", role)
	}
	if len(role.Rules) != 1 || role.Rules[0].Resources[0] != "pods" {
		t.Fatalf("role rules not copied")
	}
	if rb.Name != "bind" || rb.Namespace != "tenant-a" {
		t.Fatalf("unexpected role binding: %+v", rb)
	}
	if len(rb.Subjects) != 1 || rb.Subjects[0].Name != "svc" {
		t.Fatalf("role binding subjects incorrect")
	}
	if rb.RoleRef.Name != "role" || rb.RoleRef.Kind != "Role" {
		t.Fatalf("role binding role ref incorrect")
	}

	// mutating the returned role rules must not affect the input slice
	role.Rules[0].Resources = []string{"services"}
	if rules[0].Resources[0] != "pods" {
		t.Fatalf("expected input rules to remain unchanged")
	}

	if sa.APIVersion != corev1.SchemeGroupVersion.String() && sa.APIVersion != "" {
		t.Fatalf("unexpected apiVersion on service account: %q", sa.APIVersion)
	}
}
