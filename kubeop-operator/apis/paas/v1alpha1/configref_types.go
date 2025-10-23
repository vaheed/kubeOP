package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigRefDataType enumerates supported data sources for ConfigRef.
// +kubebuilder:validation:Enum=inline;configMap
type ConfigRefDataType string

// ConfigRefMountScope enumerates supported projection scopes.
// +kubebuilder:validation:Enum=volume;env
type ConfigRefMountScope string

// ConfigRefData defines configuration payloads shared with applications.
type ConfigRefData struct {
	// Type identifies the source of the configuration data.
	Type ConfigRefDataType `json:"type"`

	// Inline stores literal key-value pairs when Type is inline.
	// +optional
	Inline map[string]string `json:"inline,omitempty"`

	// ConfigMapRef references an existing ConfigMap when Type is configMap.
	// +optional
	ConfigMapRef string `json:"configMapRef,omitempty"`
}

// ConfigRefSpec declares configuration attachments for applications.
type ConfigRefSpec struct {
	// Data contains the configuration payload.
	Data ConfigRefData `json:"data"`

	// Scope selects how the data is presented to workloads.
	// +kubebuilder:default=volume
	Scope ConfigRefMountScope `json:"scope,omitempty"`

	// MountPath defines the path used when Scope is volume.
	// +optional
	MountPath string `json:"mountPath,omitempty"`
}

// ConfigRefStatus conveys readiness of the configuration projection.
type ConfigRefStatus struct {
	// Conditions summarise readiness for mounting or injecting the configuration.
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
// ConfigRef stores reusable configuration payloads.
type ConfigRef struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigRefSpec   `json:"spec,omitempty"`
	Status ConfigRefStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// ConfigRefList contains a list of ConfigRef resources.
type ConfigRefList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConfigRef `json:"items"`
}
