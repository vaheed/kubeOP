package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ProjectEnvironment enumerates supported project environments.
// +kubebuilder:validation:Enum=dev;stage;prod
type ProjectEnvironment string

const (
	// ProjectEnvironmentDev represents development workloads.
	ProjectEnvironmentDev ProjectEnvironment = "dev"
	// ProjectEnvironmentStage represents staging workloads prior to production cutover.
	ProjectEnvironmentStage ProjectEnvironment = "stage"
	// ProjectEnvironmentProd represents production workloads.
	ProjectEnvironmentProd ProjectEnvironment = "prod"
)

// ProjectQuotaSpec wires Kubernetes ResourceQuota and LimitRange data.
type ProjectQuotaSpec struct {
	// ResourceQuota defines limits for the project namespace.
	// +optional
	ResourceQuota *corev1.ResourceQuotaSpec `json:"resourceQuota,omitempty"`

	// LimitRange defines default requests and limits for pods/containers.
	// +optional
	LimitRange *corev1.LimitRangeSpec `json:"limitRange,omitempty"`
}

// ProjectSpec defines configuration for a tenant project.
type ProjectSpec struct {
	// TenantRef associates the project with a tenant.
	// +kubebuilder:validation:MinLength=1
	TenantRef string `json:"tenantRef"`

	// Purpose describes the business purpose of the project.
	// +kubebuilder:validation:MinLength=1
	Purpose string `json:"purpose"`

	// Environment indicates the deployment stage (dev, stage, prod).
	Environment ProjectEnvironment `json:"environment"`

	// NamespaceName declares the Kubernetes namespace managed by the project.
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	NamespaceName string `json:"namespaceName"`

	// Quotas defines Kubernetes quota and limit defaults.
	// +optional
	Quotas *ProjectQuotaSpec `json:"quotas,omitempty"`

	// PSAPreset selects the Pod Security Admission preset applied to the namespace.
	// +optional
	PSAPreset SecurityPreset `json:"psapreset,omitempty"`

	// NetworkPolicyProfileRef references predefined network policies.
	// +optional
	NetworkPolicyProfileRef string `json:"networkPolicyProfileRef,omitempty"`
}

// ProjectStatus captures observed state for a Project resource.
type ProjectStatus struct {
	// SyncNamespace indicates whether the namespace has been reconciled.
	// +optional
	SyncNamespace bool `json:"syncNs,omitempty"`

	// Conditions summarise readiness and availability.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="TENANT",type=string,JSONPath=`.spec.tenantRef`
// +kubebuilder:printcolumn:name="ENV",type=string,JSONPath=`.spec.environment`
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/tenant'])",message="metadata.labels.paas.kubeop.io/tenant is required"
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/project'])",message="metadata.labels.paas.kubeop.io/project is required"
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/app'])",message="metadata.labels.paas.kubeop.io/app is required"
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/env'])",message="metadata.labels.paas.kubeop.io/env is required"
// Project represents a tenant-scoped namespace managed by kubeOP.
type Project struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ProjectSpec   `json:"spec,omitempty"`
	Status ProjectStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// ProjectList contains a list of Project resources.
type ProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Project `json:"items"`
}
