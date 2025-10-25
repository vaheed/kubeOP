package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NetworkPolicyPreset enumerates built-in network policy templates.
// +kubebuilder:validation:Enum=deny-all;web-only;db-isolate
type NetworkPolicyPreset string

// NetworkPolicyEgressRule defines additional egress destinations.
type NetworkPolicyEgressRule struct {
	// CIDR identifies the destination CIDR block.
	// +kubebuilder:validation:Pattern=`^([0-9]{1,3}\.){3}[0-9]{1,3}/[0-9]{1,2}$`
	CIDR string `json:"cidr"`

	// Ports enumerates TCP/UDP ports allowed for the CIDR.
	// +optional
	Ports []int32 `json:"ports,omitempty"`
}

// NetworkPolicyProfileSpec declares a reusable network policy profile.
type NetworkPolicyProfileSpec struct {
	// Presets lists baseline presets applied by the profile.
	// +kubebuilder:validation:MinItems=1
	Presets []NetworkPolicyPreset `json:"presets"`

	// EgressRules defines additional egress exceptions.
	// +optional
	EgressRules []NetworkPolicyEgressRule `json:"egress,omitempty"`

	// ServicePolicy defines allowed service exposure controls for namespaces referencing the profile.
	// +optional
	ServicePolicy *ServiceExposurePolicy `json:"servicePolicy,omitempty"`
}

// ServiceExposurePolicy constrains Service types and external IP usage.
type ServiceExposurePolicy struct {
	// AllowedTypes enumerates permitted Kubernetes Service types. ClusterIP is implicitly allowed even when omitted.
	// +optional
	AllowedTypes []corev1.ServiceType `json:"allowedTypes,omitempty"`

	// AllowedExternalIPs restricts which external IPs can be requested. Leave empty to forbid external IP usage.
	// +optional
	AllowedExternalIPs []string `json:"allowedExternalIPs,omitempty"`
}

// NetworkPolicyProfileStatus reports readiness information for policy profiles.
type NetworkPolicyProfileStatus struct {
	// Conditions summarise policy validation status.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
// NetworkPolicyProfile defines reusable network policy templates.
type NetworkPolicyProfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkPolicyProfileSpec   `json:"spec,omitempty"`
	Status NetworkPolicyProfileStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// NetworkPolicyProfileList contains a list of NetworkPolicyProfile resources.
type NetworkPolicyProfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkPolicyProfile `json:"items"`
}
