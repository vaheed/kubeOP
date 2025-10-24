package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecurityPreset enumerates supported PodSecurity profiles.
// +kubebuilder:validation:Enum=baseline;restricted
// +kubebuilder:default=baseline
type SecurityPreset string

const (
	// SecurityPresetBaseline maps to the Kubernetes baseline PodSecurity profile.
	SecurityPresetBaseline SecurityPreset = "baseline"
	// SecurityPresetRestricted maps to the Kubernetes restricted PodSecurity profile.
	SecurityPresetRestricted SecurityPreset = "restricted"
)

// RuntimeClassProfileSpec defines runtime class defaults for workloads.
type RuntimeClassProfileSpec struct {
	// RuntimeClassName identifies the Kubernetes RuntimeClass to reference.
	// +kubebuilder:validation:MinLength=1
	RuntimeClassName string `json:"runtimeClassName"`

	// Tolerations applies node tolerations to workloads selecting the profile.
	// +optional
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`

	// NodeSelector restricts nodes for workloads selecting the profile.
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// SecurityPreset selects the PodSecurity standard enforced for workloads.
	// +optional
	SecurityPreset SecurityPreset `json:"securityPreset,omitempty"`
}

// RuntimeClassProfileStatus reports readiness and validation information.
type RuntimeClassProfileStatus struct {
	// Conditions exposes readiness signals for the profile.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="RUNTIME",type=string,JSONPath=`.spec.runtimeClassName`
// +kubebuilder:printcolumn:name="SECURITY",type=string,JSONPath=`.spec.securityPreset`
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
// RuntimeClassProfile encapsulates runtime settings shared across workloads.
type RuntimeClassProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RuntimeClassProfileSpec   `json:"spec,omitempty"`
	Status RuntimeClassProfileStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// RuntimeClassProfileList contains a list of RuntimeClassProfile resources.
type RuntimeClassProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RuntimeClassProfile `json:"items"`
}
