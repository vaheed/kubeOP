package v1alpha1

import (
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CacheEngine enumerates supported cache engines.
// +kubebuilder:validation:Enum=redis;memcached
type CacheEngine string

// CachePlan defines sizing information for cache instances.
type CachePlan struct {
        // Size identifies the cache tier (e.g., small, medium, large).
        // +kubebuilder:validation:MinLength=1
        Size string `json:"size"`
}

// CacheInstanceSpec declares cache provisioning configuration.
type CacheInstanceSpec struct {
        // Engine selects the cache engine to provision.
        Engine CacheEngine `json:"engine"`

        // Plan selects the sizing plan for the cache.
        Plan CachePlan `json:"plan"`
}

// CacheInstanceStatus reports runtime details for a cache instance.
type CacheInstanceStatus struct {
        // Endpoint exposes the access endpoint.
        // +optional
        Endpoint string `json:"endpoint,omitempty"`

        // Conditions summarise provisioning status.
        // +optional
        Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="ENGINE",type=string,JSONPath=`.spec.engine`
// +kubebuilder:printcolumn:name="PLAN",type=string,JSONPath=`.spec.plan.size`
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/tenant'])",message="metadata.labels.paas.kubeop.io/tenant is required"
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/project'])",message="metadata.labels.paas.kubeop.io/project is required"
// CacheInstance provisions managed cache services.
type CacheInstance struct {
        metav1.TypeMeta   `json:",inline"`
        metav1.ObjectMeta `json:"metadata,omitempty"`

        Spec   CacheInstanceSpec   `json:"spec,omitempty"`
        Status CacheInstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// CacheInstanceList contains a list of CacheInstance resources.
type CacheInstanceList struct {
        metav1.TypeMeta `json:",inline"`
        metav1.ListMeta `json:"metadata,omitempty"`
        Items           []CacheInstance `json:"items"`
}
