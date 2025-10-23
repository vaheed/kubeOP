package v1alpha1

import (
        corev1 "k8s.io/api/core/v1"
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
        "k8s.io/apimachinery/pkg/api/resource"
)

// ResourceLimitThreshold captures soft and hard quota values for a metric.
type ResourceLimitThreshold struct {
        // Soft represents an advisory threshold that triggers warnings but does not block usage.
        // +optional
        Soft *resource.Quantity `json:"soft,omitempty"`

        // Hard is the enforced limit that cannot be exceeded.
        // +kubebuilder:validation:Required
        Hard resource.Quantity `json:"hard"`
}

// TenantQuotaSpec defines quota settings for a tenant across key metrics.
type TenantQuotaSpec struct {
        // CPU represents vCPU limits for workloads under the tenant.
        // +optional
        CPU *ResourceLimitThreshold `json:"cpu,omitempty"`

        // Memory represents memory limits for workloads under the tenant.
        // +optional
        Memory *ResourceLimitThreshold `json:"memory,omitempty"`

        // Storage captures persistent volume capacity allocations.
        // +optional
        Storage *ResourceLimitThreshold `json:"storage,omitempty"`

        // Objects reflects Kubernetes object count quotas.
        // +optional
        Objects *ResourceLimitThreshold `json:"objects,omitempty"`
}

// TenantUsageStatus tracks usage metrics for a tenant.
type TenantUsageStatus struct {
        // CPU reports the current CPU consumption recorded for the tenant.
        // +optional
        CPU *resource.Quantity `json:"cpu,omitempty"`

        // Memory reports the current memory consumption recorded for the tenant.
        // +optional
        Memory *resource.Quantity `json:"memory,omitempty"`

        // Storage captures used persistent storage for the tenant.
        // +optional
        Storage *resource.Quantity `json:"storage,omitempty"`

        // Objects indicates the current number of Kubernetes objects for the tenant.
        // +optional
        Objects *int64 `json:"objects,omitempty"`
}

// TenantSpec defines the desired state of a Tenant.
type TenantSpec struct {
        // DisplayName is a human friendly identifier for the tenant.
        // +kubebuilder:validation:MinLength=1
        DisplayName string `json:"displayName"`

        // BillingAccountRef references the external billing account for the tenant.
        // +kubebuilder:validation:MinLength=1
        BillingAccountRef string `json:"billingAccountRef"`

        // PolicyRefs lists policy documents that apply to the tenant.
        // +optional
        PolicyRefs []string `json:"policyRefs,omitempty"`

        // Quotas defines usage limits for the tenant.
        // +optional
        Quotas *TenantQuotaSpec `json:"quotas,omitempty"`
}

// TenantStatus captures observed information for a Tenant.
type TenantStatus struct {
        // Usage reports the aggregated resource consumption for the tenant.
        // +optional
        Usage *TenantUsageStatus `json:"usage,omitempty"`

        // Conditions reports the reconciliation status for the tenant.
        // +optional
        Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="DISPLAY",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="BILLING",type=string,JSONPath=`.spec.billingAccountRef`
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/tenant'])",message="metadata.labels.paas.kubeop.io/tenant is required"
// +kubebuilder:validation:XValidation:rule="!has(oldSelf) || self.spec.billingAccountRef == oldSelf.spec.billingAccountRef",message="billingAccountRef is immutable"
// Tenant represents an organisation consuming kubeOP services.
type Tenant struct {
        metav1.TypeMeta   `json:",inline"`
        metav1.ObjectMeta `json:"metadata,omitempty"`

        Spec   TenantSpec   `json:"spec,omitempty"`
        Status TenantStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// TenantList contains a list of Tenant resources.
type TenantList struct {
        metav1.TypeMeta `json:",inline"`
        metav1.ListMeta `json:"metadata,omitempty"`
        Items           []Tenant `json:"items"`
}

// TenantQuotaDefaults describes optional default LimitRange settings for a tenant namespace.
type TenantQuotaDefaults struct {
        // Requests defines default resource requests applied via LimitRange.
        // +optional
        Requests corev1.ResourceList `json:"requests,omitempty"`

        // Limits defines default resource limits applied via LimitRange.
        // +optional
        Limits corev1.ResourceList `json:"limits,omitempty"`
}
