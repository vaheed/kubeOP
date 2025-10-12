package service

import (
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubeop/internal/config"
)

// BuildTenantNetworkPolicies returns the default network policies applied to
// dedicated tenant namespaces. It always includes a default deny and DNS egress
// policy and optionally an ingress allow-list if configured.
func BuildTenantNetworkPolicies(cfg *config.Config, namespace string) []*netv1.NetworkPolicy {
	if cfg == nil {
		cfg = &config.Config{}
	}
	npDeny := &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "default-deny", Namespace: namespace}}
	npDeny.Spec.PodSelector = metav1.LabelSelector{}
	npDeny.Spec.PolicyTypes = []netv1.PolicyType{netv1.PolicyTypeIngress, netv1.PolicyTypeEgress}

	npDNS := &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-dns", Namespace: namespace}}
	npDNS.Spec.PodSelector = metav1.LabelSelector{}
	npDNS.Spec.PolicyTypes = []netv1.PolicyType{netv1.PolicyTypeEgress}
	npDNS.Spec.Egress = []netv1.NetworkPolicyEgressRule{{
		To: []netv1.NetworkPolicyPeer{{
			NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{cfg.DNSNamespaceLabelKey: cfg.DNSNamespaceLabelValue}},
			PodSelector:       &metav1.LabelSelector{MatchLabels: map[string]string{cfg.DNSPodLabelKey: cfg.DNSPodLabelValue}},
		}},
		Ports: []netv1.NetworkPolicyPort{{Protocol: protoPtr(corev1.ProtocolUDP), Port: intstrPtr(53)}},
	}}

	policies := []*netv1.NetworkPolicy{npDeny, npDNS}

	if cfg.IngressNamespaceLabelKey != "" && cfg.IngressNamespaceLabelValue != "" {
		npIngress := &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-from-ingress", Namespace: namespace}}
		npIngress.Spec.PodSelector = metav1.LabelSelector{}
		npIngress.Spec.PolicyTypes = []netv1.PolicyType{netv1.PolicyTypeIngress}
		npIngress.Spec.Ingress = []netv1.NetworkPolicyIngressRule{{
			From: []netv1.NetworkPolicyPeer{{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{cfg.IngressNamespaceLabelKey: cfg.IngressNamespaceLabelValue}}}},
		}}
		policies = append(policies, npIngress)
	}

	return policies
}

// BuildNamespaceRBAC returns a service account, role, and binding tuple for the
// provided namespace and logical names.
func BuildNamespaceRBAC(namespace, saName, roleName, rbName string, rules []rbacv1.PolicyRule) (*corev1.ServiceAccount, *rbacv1.Role, *rbacv1.RoleBinding) {
	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: saName, Namespace: namespace}}
	role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: roleName, Namespace: namespace}}
	role.Rules = append([]rbacv1.PolicyRule{}, rules...)
	rb := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: rbName, Namespace: namespace}}
	rb.Subjects = []rbacv1.Subject{{Kind: "ServiceAccount", Name: sa.Name, Namespace: namespace}}
	rb.RoleRef = rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: role.Name}
	return sa, role, rb
}
