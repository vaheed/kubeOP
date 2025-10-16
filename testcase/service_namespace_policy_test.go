package testcase

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"kubeop/internal/config"
	"kubeop/internal/service"
)

func newNamespacePolicyConfig() *config.Config {
	return &config.Config{
		NamespaceQuotaRequestsCPU:                   "2",
		NamespaceQuotaLimitsCPU:                     "4",
		NamespaceQuotaRequestsMemory:                "4Gi",
		NamespaceQuotaLimitsMemory:                  "8Gi",
		NamespaceQuotaRequestsEphemeral:             "10Gi",
		NamespaceQuotaLimitsEphemeral:               "20Gi",
		NamespaceQuotaPods:                          "30",
		NamespaceQuotaServices:                      "10",
		NamespaceQuotaServicesLoadBalancers:         "1",
		NamespaceQuotaConfigMaps:                    "100",
		NamespaceQuotaSecrets:                       "100",
		NamespaceQuotaPVCs:                          "10",
		NamespaceQuotaRequestsStorage:               "200Gi",
		NamespaceQuotaDeployments:                   "20",
		NamespaceQuotaReplicaSets:                   "40",
		NamespaceQuotaStatefulSets:                  "5",
		NamespaceQuotaJobs:                          "20",
		NamespaceQuotaCronJobs:                      "10",
		NamespaceQuotaIngresses:                     "10",
		NamespaceQuotaScopes:                        "NotBestEffort",
		NamespaceQuotaPriorityClasses:               "",
		NamespaceLRContainerMaxCPU:                  "2",
		NamespaceLRContainerMaxMemory:               "2Gi",
		NamespaceLRContainerMinCPU:                  "100m",
		NamespaceLRContainerMinMemory:               "128Mi",
		NamespaceLRContainerDefaultCPU:              "500m",
		NamespaceLRContainerDefaultMemory:           "512Mi",
		NamespaceLRContainerDefaultRequestCPU:       "300m",
		NamespaceLRContainerDefaultRequestMemory:    "256Mi",
		NamespaceLRContainerMaxEphemeral:            "2Gi",
		NamespaceLRContainerMinEphemeral:            "128Mi",
		NamespaceLRContainerDefaultEphemeral:        "512Mi",
		NamespaceLRContainerDefaultRequestEphemeral: "256Mi",
		NamespaceLRExtMin:                           "",
		NamespaceLRExtDefault:                       "",
		NamespaceLRExtDefaultRequest:                "",
	}
}

func TestNamespaceLimitPolicyDefaults(t *testing.T) {
	cfg := newNamespacePolicyConfig()

	hard := service.TestDefaultQuota(cfg, nil)
	if got := quantityString(hard, corev1.ResourceRequestsCPU); got != "2" {
		t.Fatalf("expected requests cpu 2, got %s", got)
	}
	if got := quantityString(hard, corev1.ResourceLimitsMemory); got != "8Gi" {
		t.Fatalf("expected limits memory 8Gi, got %s", got)
	}
	if got := quantityString(hard, corev1.ResourceName("services.loadbalancers")); got != "1" {
		t.Fatalf("expected lb quota 1, got %s", got)
	}

	overrides := map[string]string{"pods": "5", "services.loadbalancers": "2"}
	hard = service.TestDefaultQuota(cfg, overrides)
	if got := quantityString(hard, corev1.ResourcePods); got != "5" {
		t.Fatalf("expected pods override 5, got %s", got)
	}
	if got := quantityString(hard, corev1.ResourceName("services.loadbalancers")); got != "2" {
		t.Fatalf("expected lb override 2, got %s", got)
	}

	limits := service.TestDefaultLimitRange(cfg)
	if len(limits) != 2 {
		t.Fatalf("expected two limit range items (container, pod), got %d", len(limits))
	}
	var container, pod corev1.LimitRangeItem
	for _, item := range limits {
		switch item.Type {
		case corev1.LimitTypeContainer:
			container = item
		case corev1.LimitTypePod:
			pod = item
		}
	}
	if container.Type != corev1.LimitTypeContainer {
		t.Fatalf("missing container limit item: %#v", limits)
	}
	if pod.Type != corev1.LimitTypePod {
		t.Fatalf("missing pod limit item: %#v", limits)
	}
	if got := quantityString(container.DefaultRequest, corev1.ResourceCPU); got != "300m" {
		t.Fatalf("expected container default request cpu 300m, got %s", got)
	}
	if got := quantityString(container.Default, corev1.ResourceEphemeralStorage); got != "512Mi" {
		t.Fatalf("expected container default ephemeral 512Mi, got %s", got)
	}
	if hasResource(container.Max, corev1.ResourceName("example.com/device")) {
		t.Fatalf("expected no gpu max by default")
	}
	if got := quantityString(pod.Max, corev1.ResourceCPU); got != "4" {
		t.Fatalf("expected pod max cpu 4, got %s", got)
	}
	if got := quantityString(pod.Min, corev1.ResourceEphemeralStorage); got != "128Mi" {
		t.Fatalf("expected pod min ephemeral 128Mi, got %s", got)
	}
}

func TestConfigureNamespaceResourceQuotaScopesFiltered(t *testing.T) {
	cfg := newNamespacePolicyConfig()
	cfg.NamespaceQuotaScopes = "NotBestEffort,BestEffort"
	cfg.NamespaceQuotaPriorityClasses = "tenant-high,tenant-low"

	rq := &corev1.ResourceQuota{}
	service.TestConfigureNamespaceResourceQuota(cfg, rq, nil)

	if len(rq.Spec.Scopes) != 0 {
		t.Fatalf("expected incompatible scopes to be dropped, got %#v", rq.Spec.Scopes)
	}
	if rq.Spec.ScopeSelector == nil || len(rq.Spec.ScopeSelector.MatchExpressions) != 1 {
		t.Fatalf("expected priority class selector, got %#v", rq.Spec.ScopeSelector)
	}
	expr := rq.Spec.ScopeSelector.MatchExpressions[0]
	if expr.ScopeName != corev1.ResourceQuotaScopePriorityClass {
		t.Fatalf("expected priority class scope, got %s", expr.ScopeName)
	}
	if expr.Operator != corev1.ScopeSelectorOpIn {
		t.Fatalf("expected IN operator, got %s", expr.Operator)
	}
	if len(expr.Values) != 2 || expr.Values[0] != "tenant-high" || expr.Values[1] != "tenant-low" {
		t.Fatalf("expected sorted priority class values, got %#v", expr.Values)
	}
}

func TestConfigureNamespaceResourceQuotaScopesRetainedWhenSupported(t *testing.T) {
	cfg := newNamespacePolicyConfig()
	cfg.NamespaceQuotaPods = ""
	cfg.NamespaceQuotaServices = ""
	cfg.NamespaceQuotaServicesLoadBalancers = ""
	cfg.NamespaceQuotaConfigMaps = ""
	cfg.NamespaceQuotaSecrets = ""
	cfg.NamespaceQuotaPVCs = ""
	cfg.NamespaceQuotaRequestsStorage = ""
	cfg.NamespaceQuotaDeployments = ""
	cfg.NamespaceQuotaReplicaSets = ""
	cfg.NamespaceQuotaStatefulSets = ""
	cfg.NamespaceQuotaJobs = ""
	cfg.NamespaceQuotaCronJobs = ""
	cfg.NamespaceQuotaIngresses = ""
	cfg.NamespaceQuotaScopes = "NotBestEffort"

	rq := &corev1.ResourceQuota{}
	service.TestConfigureNamespaceResourceQuota(cfg, rq, nil)

	if len(rq.Spec.Scopes) != 1 {
		t.Fatalf("expected supported scope to be retained, got %#v", rq.Spec.Scopes)
	}
	if rq.Spec.Scopes[0] != corev1.ResourceQuotaScope("NotBestEffort") {
		t.Fatalf("expected NotBestEffort scope, got %#v", rq.Spec.Scopes)
	}
}

func TestBuildNamespaceLimitPolicyObjects(t *testing.T) {
	cfg := newNamespacePolicyConfig()

	rq, lr, err := service.TestBuildNamespaceLimitPolicyObjects(cfg, "tenant-demo", map[string]string{"pods": "12"})
	if err != nil {
		t.Fatalf("expected no error building policy, got %v", err)
	}
	if rq.Name != "tenant-quota" {
		t.Fatalf("expected tenant-quota name, got %s", rq.Name)
	}
	if rq.Annotations["managed-by"] != "kubeop-operator" {
		t.Fatalf("expected managed-by annotation, got %#v", rq.Annotations)
	}
	if got := quantityString(rq.Spec.Hard, corev1.ResourcePods); got != "12" {
		t.Fatalf("expected pod override 12, got %s", got)
	}
	if lr.Name != "tenant-limits" {
		t.Fatalf("expected tenant-limits name, got %s", lr.Name)
	}
	if lr.Annotations["managed-by"] != "kubeop-operator" {
		t.Fatalf("expected managed-by annotation on limitrange, got %#v", lr.Annotations)
	}
	if len(lr.Spec.Limits) != 2 {
		t.Fatalf("expected two limit items, got %d", len(lr.Spec.Limits))
	}
}

func TestBuildNamespaceLimitPolicyObjectsRequiresNamespace(t *testing.T) {
	cfg := newNamespacePolicyConfig()
	if _, _, err := service.TestBuildNamespaceLimitPolicyObjects(cfg, " ", nil); err == nil {
		t.Fatalf("expected error when namespace is empty")
	}
}

func TestNamespaceLimitPolicyExtendedResourcesOptIn(t *testing.T) {
	cfg := newNamespacePolicyConfig()
	cfg.NamespaceLRExtMax = "example.com/device=1"

	limits := service.TestDefaultLimitRange(cfg)
	if len(limits) != 2 {
		t.Fatalf("expected two limit range items, got %d", len(limits))
	}
	var container, pod corev1.LimitRangeItem
	for _, item := range limits {
		switch item.Type {
		case corev1.LimitTypeContainer:
			container = item
		case corev1.LimitTypePod:
			pod = item
		}
	}
	if got := quantityString(container.Max, corev1.ResourceName("example.com/device")); got != "1" {
		t.Fatalf("expected gpu max 1 when configured, got %s", got)
	}
	if got := quantityString(pod.Max, corev1.ResourceName("example.com/device")); got != "1" {
		t.Fatalf("expected pod gpu max 1 when configured, got %s", got)
	}
}

func quantityString(list corev1.ResourceList, name corev1.ResourceName) string {
	qty := list[name]
	return qty.String()
}

func hasResource(list corev1.ResourceList, name corev1.ResourceName) bool {
	if list == nil {
		return false
	}
	_, ok := list[name]
	return ok
}
