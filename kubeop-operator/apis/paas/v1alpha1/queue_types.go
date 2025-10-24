package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// QueueEngine enumerates supported message queue engines.
// +kubebuilder:validation:Enum=rabbitmq;sqs-compat
type QueueEngine string

// QueuePlan defines sizing attributes for queue instances.
type QueuePlan struct {
	// Size identifies the plan tier for the queue.
	// +kubebuilder:validation:MinLength=1
	Size string `json:"size"`
}

// QueueInstanceSpec declares configuration for queue services.
type QueueInstanceSpec struct {
	// Engine selects the queue engine to provision.
	Engine QueueEngine `json:"engine"`

	// Plan selects the queue plan.
	Plan QueuePlan `json:"plan"`
}

// QueueInstanceStatus exposes runtime details for queue instances.
type QueueInstanceStatus struct {
	// Endpoint exposes the queue connection endpoint.
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
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/app'])",message="metadata.labels.paas.kubeop.io/app is required"
// QueueInstance provisions managed queue services.
type QueueInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   QueueInstanceSpec   `json:"spec,omitempty"`
	Status QueueInstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// QueueInstanceList contains a list of QueueInstance resources.
type QueueInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []QueueInstance `json:"items"`
}
