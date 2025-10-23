package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DatabaseEngine enumerates supported database engines.
// +kubebuilder:validation:Enum=pg;mysql
type DatabaseEngine string

// DatabasePlan defines sizing information for database instances.
type DatabasePlan struct {
	// Size identifies the plan size (e.g., small, medium, large).
	// +kubebuilder:validation:MinLength=1
	Size string `json:"size"`

	// IOPS describes provisioned I/O capacity.
	// +optional
	IOPS int32 `json:"iops,omitempty"`
}

// DatabaseInstanceSpec defines database provisioning configuration.
type DatabaseInstanceSpec struct {
	// Engine selects the database engine to provision.
	Engine DatabaseEngine `json:"engine"`

	// Plan selects the sizing plan for the database.
	Plan DatabasePlan `json:"plan"`

	// BackupPolicyRef references a backup policy configuration.
	// +optional
	BackupPolicyRef string `json:"backupPolicyRef,omitempty"`

	// ConnectionSecretRef references a secret where connection info is stored.
	// +kubebuilder:validation:MinLength=1
	ConnectionSecretRef string `json:"connSecretRef"`
}

// DatabaseInstanceStatus reports runtime details for a database instance.
type DatabaseInstanceStatus struct {
	// Endpoint exposes the connection endpoint for the instance.
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
// DatabaseInstance provisions managed relational databases.
type DatabaseInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DatabaseInstanceSpec   `json:"spec,omitempty"`
	Status DatabaseInstanceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// DatabaseInstanceList contains a list of DatabaseInstance resources.
type DatabaseInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DatabaseInstance `json:"items"`
}
