package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretRefDataType enumerates supported secret sources.
// +kubebuilder:validation:Enum=inline;secret
type SecretRefDataType string

// SecretRefScope enumerates supported injection scopes for secrets.
// +kubebuilder:validation:Enum=volume;env;file
type SecretRefScope string

// SecretRefData defines secret payloads shared with applications.
type SecretRefData struct {
	// Type identifies the data source for the secret payload.
	Type SecretRefDataType `json:"type"`

	// Inline stores base64 encoded values for inline secrets.
	// +optional
	Inline map[string]string `json:"inline,omitempty"`

	// SecretRef references an existing Kubernetes Secret when Type is secret.
	// +optional
	SecretRef string `json:"secretRef,omitempty"`
}

// SecretRefSpec declares secret attachments for applications.
type SecretRefSpec struct {
	// Data contains the secret payload.
	Data SecretRefData `json:"data"`

	// Scope selects how the secret is exposed to workloads.
	// +kubebuilder:default=env
	Scope SecretRefScope `json:"scope,omitempty"`

	// MountPath is required when Scope is volume or file.
	// +optional
	MountPath string `json:"mountPath,omitempty"`
}

// SecretRefStatus communicates readiness for secret consumption.
type SecretRefStatus struct {
	// Conditions summarise readiness for projecting the secret.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="TYPE",type=string,JSONPath=`.spec.data.type`
// +kubebuilder:printcolumn:name="SCOPE",type=string,JSONPath=`.spec.scope`
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/tenant'])",message="metadata.labels.paas.kubeop.io/tenant is required"
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/project'])",message="metadata.labels.paas.kubeop.io/project is required"
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/app'])",message="metadata.labels.paas.kubeop.io/app is required"
// SecretRef stores reusable secret payloads.
type SecretRef struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   SecretRefSpec   `json:"spec,omitempty"`
	Status SecretRefStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// SecretRefList contains a list of SecretRef resources.
type SecretRefList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []SecretRef `json:"items"`
}
