package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RegistryCredentialType enumerates supported registry backends.
// +kubebuilder:validation:Enum=dockerHub;ecr;gcr;harbor
type RegistryCredentialType string

const (
	// RegistryCredentialDockerHub represents Docker Hub credentials.
	RegistryCredentialDockerHub RegistryCredentialType = "dockerHub"
	// RegistryCredentialECR represents AWS Elastic Container Registry credentials.
	RegistryCredentialECR RegistryCredentialType = "ecr"
	// RegistryCredentialGCR represents Google Container Registry credentials.
	RegistryCredentialGCR RegistryCredentialType = "gcr"
	// RegistryCredentialHarbor represents Harbor registry credentials.
	RegistryCredentialHarbor RegistryCredentialType = "harbor"
)

// RegistryCredentialSpec defines credentials for accessing a container registry.
type RegistryCredentialSpec struct {
	// Type identifies the registry provider.
	Type RegistryCredentialType `json:"type"`

	// SecretRef references the Kubernetes secret containing credential data.
	// +kubebuilder:validation:MinLength=1
	SecretRef string `json:"secretRef"`

	// TenantRef links the credential to a tenant scope.
	// +kubebuilder:validation:MinLength=1
	TenantRef string `json:"tenantRef"`
}

// RegistryCredentialStatus tracks reconciliation details for registry credentials.
type RegistryCredentialStatus struct {
	// Conditions indicate credential readiness.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="TYPE",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="TENANT",type=string,JSONPath=`.spec.tenantRef`
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/tenant'])",message="metadata.labels.paas.kubeop.io/tenant is required"
// RegistryCredential stores secret references for container registry access.
type RegistryCredential struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RegistryCredentialSpec   `json:"spec,omitempty"`
	Status RegistryCredentialStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// RegistryCredentialList contains a list of RegistryCredential resources.
type RegistryCredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RegistryCredential `json:"items"`
}
