package v1alpha1

import (
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceBindingConsumerType enumerates supported binding consumers.
// +kubebuilder:validation:Enum=app;serviceAccount
type ServiceBindingConsumerType string

// ServiceBindingProviderType enumerates supported binding providers.
// +kubebuilder:validation:Enum=database;cache;queue
type ServiceBindingProviderType string

// ServiceBindingInjectionType enumerates how bindings are exposed.
// +kubebuilder:validation:Enum=env;file;secret
type ServiceBindingInjectionType string

// ServiceBindingConsumer references the binding consumer.
type ServiceBindingConsumer struct {
        // Type identifies the consumer type (App or ServiceAccount).
        Type ServiceBindingConsumerType `json:"type"`

        // Name references the consumer resource name.
        // +kubebuilder:validation:MinLength=1
        Name string `json:"name"`
}

// ServiceBindingProvider references the binding provider.
type ServiceBindingProvider struct {
        // Type identifies the provider category (database, cache, queue).
        Type ServiceBindingProviderType `json:"type"`

        // Name references the provider resource name.
        // +kubebuilder:validation:MinLength=1
        Name string `json:"name"`
}

// ServiceBindingSpec declares service binding configuration.
type ServiceBindingSpec struct {
        // Consumer identifies the binding consumer.
        Consumer ServiceBindingConsumer `json:"consumerRef"`

        // Provider identifies the backing service instance.
        Provider ServiceBindingProvider `json:"providerRef"`

        // InjectAs selects how the binding is projected into the workload.
        InjectAs ServiceBindingInjectionType `json:"injectAs"`
}

// ServiceBindingStatus reports binding propagation status.
type ServiceBindingStatus struct {
        // Conditions summarise binding readiness.
        // +optional
        Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="CONSUMER",type=string,JSONPath=`.spec.consumerRef.name`
// +kubebuilder:printcolumn:name="PROVIDER",type=string,JSONPath=`.spec.providerRef.name`
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/tenant'])",message="metadata.labels.paas.kubeop.io/tenant is required"
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/project'])",message="metadata.labels.paas.kubeop.io/project is required"
// ServiceBinding links workloads to managed services.
type ServiceBinding struct {
        metav1.TypeMeta   `json:",inline"`
        metav1.ObjectMeta `json:"metadata,omitempty"`

        Spec   ServiceBindingSpec   `json:"spec,omitempty"`
        Status ServiceBindingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// ServiceBindingList contains a list of ServiceBinding resources.
type ServiceBindingList struct {
        metav1.TypeMeta `json:",inline"`
        metav1.ListMeta `json:"metadata,omitempty"`
        Items           []ServiceBinding `json:"items"`
}
