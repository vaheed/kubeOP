package v1alpha1

import (
        "k8s.io/apimachinery/pkg/api/resource"
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MetricQuotaTarget enumerates quota subjects.
// +kubebuilder:validation:Enum=app;project;tenant
type MetricQuotaTarget string

// MetricQuotaSpec defines metered limits for workloads.
type MetricQuotaSpec struct {
        // Target identifies the resource subject to the quota.
        Target MetricQuotaTarget `json:"target"`

        // CPU defines the CPU usage limit.
        // +optional
        CPU *resource.Quantity `json:"cpu,omitempty"`

        // Memory defines the memory usage limit.
        // +optional
        Memory *resource.Quantity `json:"memory,omitempty"`

        // Egress defines the network egress limit.
        // +optional
        Egress *resource.Quantity `json:"egress,omitempty"`

        // LBHours defines the load balancer hours limit.
        // +optional
        LBHours *resource.Quantity `json:"lbHours,omitempty"`
}

// MetricQuotaStatus reports current usage for metered resources.
type MetricQuotaStatus struct {
        // Current records the current resource usage snapshot.
        // +optional
        Current map[string]resource.Quantity `json:"current,omitempty"`

        // Conditions summarise quota enforcement status.
        // +optional
        Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="TARGET",type=string,JSONPath=`.spec.target`
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/tenant'])",message="metadata.labels.paas.kubeop.io/tenant is required"
// MetricQuota defines metered usage limits.
type MetricQuota struct {
        metav1.TypeMeta   `json:",inline"`
        metav1.ObjectMeta `json:"metadata,omitempty"`

        Spec   MetricQuotaSpec   `json:"spec,omitempty"`
        Status MetricQuotaStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// MetricQuotaList contains a list of MetricQuota resources.
type MetricQuotaList struct {
        metav1.TypeMeta `json:",inline"`
        metav1.ListMeta `json:"metadata,omitempty"`
        Items           []MetricQuota `json:"items"`
}
