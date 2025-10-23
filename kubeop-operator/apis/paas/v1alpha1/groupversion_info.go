package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

var (
	// GroupVersion identifies the API group and version for kubeOP custom resources.
	GroupVersion = schema.GroupVersion{Group: "paas.kubeop.io", Version: "v1alpha1"}

	// SchemeBuilder accumulates functions that register the kubeOP operator types with a scheme.
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}

	// AddToScheme applies all registered types to the provided scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func init() {
	SchemeBuilder.Register(
		&Tenant{}, &TenantList{},
		&Domain{}, &DomainList{},
		&RegistryCredential{}, &RegistryCredentialList{},
		&AlertPolicy{}, &AlertPolicyList{},
		&BillingPlan{}, &BillingPlanList{},
		&RuntimeClassProfile{}, &RuntimeClassProfileList{},
		&Project{}, &ProjectList{},
		&App{}, &AppList{},
		&AppRelease{}, &AppReleaseList{},
		&ConfigRef{}, &ConfigRefList{},
		&SecretRef{}, &SecretRefList{},
		&IngressRoute{}, &IngressRouteList{},
		&CertificateRequest{}, &CertificateRequestList{},
		&Job{}, &JobList{},
		&DatabaseInstance{}, &DatabaseInstanceList{},
		&CacheInstance{}, &CacheInstanceList{},
		&QueueInstance{}, &QueueInstanceList{},
		&Bucket{}, &BucketList{},
		&BucketPolicy{}, &BucketPolicyList{},
		&ServiceBinding{}, &ServiceBindingList{},
		&NetworkPolicyProfile{}, &NetworkPolicyProfileList{},
		&MetricQuota{}, &MetricQuotaList{},
		&BillingUsage{}, &BillingUsageList{},
		&Invoice{}, &InvoiceList{},
	)
}
