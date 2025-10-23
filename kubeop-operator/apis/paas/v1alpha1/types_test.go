package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestAddToSchemeRegistersResources(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme returned error: %v", err)
	}

	kinds := []string{
		"Tenant", "Domain", "RegistryCredential", "AlertPolicy", "BillingPlan",
		"RuntimeClassProfile", "Project", "App", "AppRelease", "ConfigRef", "SecretRef",
		"IngressRoute", "CertificateRequest", "Job", "DatabaseInstance", "CacheInstance",
		"QueueInstance", "Bucket", "BucketPolicy", "ServiceBinding", "NetworkPolicyProfile",
		"MetricQuota", "BillingUsage", "Invoice",
	}
	for _, kind := range kinds {
		if _, err := scheme.New(GroupVersion.WithKind(kind)); err != nil {
			t.Fatalf("expected kind %s to be registered: %v", kind, err)
		}
	}
}
