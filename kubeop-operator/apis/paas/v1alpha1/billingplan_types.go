package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BillingMeters toggles metering for supported resources.
type BillingMeters struct {
	// CPU enables metering for CPU usage.
	// +optional
	CPU bool `json:"cpu,omitempty"`

	// Memory enables metering for memory usage.
	// +optional
	Memory bool `json:"mem,omitempty"`

	// Storage enables metering for persistent storage.
	// +optional
	Storage bool `json:"storage,omitempty"`

	// Egress enables metering for network egress traffic.
	// +optional
	Egress bool `json:"egress,omitempty"`

	// LBHours enables metering for load balancer usage.
	// +optional
	LBHours bool `json:"lbHours,omitempty"`

	// IPs enables metering for static IPs.
	// +optional
	IPs bool `json:"ips,omitempty"`

	// Objects enables metering for Kubernetes object counts.
	// +optional
	Objects bool `json:"objects,omitempty"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *BillingMeters) DeepCopyInto(out *BillingMeters) {
	*out = *in
}

// DeepCopy creates a new BillingMeters instance.
func (in *BillingMeters) DeepCopy() *BillingMeters {
	if in == nil {
		return nil
	}
	out := new(BillingMeters)
	in.DeepCopyInto(out)
	return out
}

// BillingRates captures per-unit pricing for each meter.
type BillingRates struct {
	// CPURate identifies the price per CPU unit.
	// +optional
	CPU string `json:"cpu,omitempty"`

	// MemoryRate identifies the price per GiB of memory.
	// +optional
	Memory string `json:"mem,omitempty"`

	// StorageRate identifies the price per GiB of storage.
	// +optional
	Storage string `json:"storage,omitempty"`

	// EgressRate identifies the price per GiB of egress traffic.
	// +optional
	Egress string `json:"egress,omitempty"`

	// LBHoursRate identifies the price per load balancer hour.
	// +optional
	LBHours string `json:"lbHours,omitempty"`

	// IPsRate identifies the price per static IP.
	// +optional
	IPs string `json:"ips,omitempty"`

	// ObjectsRate identifies the price per object.
	// +optional
	Objects string `json:"objects,omitempty"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *BillingRates) DeepCopyInto(out *BillingRates) {
	*out = *in
}

// DeepCopy creates a new BillingRates instance.
func (in *BillingRates) DeepCopy() *BillingRates {
	if in == nil {
		return nil
	}
	out := new(BillingRates)
	in.DeepCopyInto(out)
	return out
}

// BillingPlanSpec declares metering behaviour for tenants and projects.
type BillingPlanSpec struct {
	// Meters enables resource consumption tracking.
	// +optional
	Meters *BillingMeters `json:"meters,omitempty"`

	// Rates declares the currency rates for enabled meters.
	// +optional
	Rates *BillingRates `json:"rates,omitempty"`

	// Currency specifies the ISO 4217 currency code used for billing.
	// +kubebuilder:validation:Pattern=`^[A-Z]{3}$`
	Currency string `json:"currency"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *BillingPlanSpec) DeepCopyInto(out *BillingPlanSpec) {
	*out = *in
	if in.Meters != nil {
		out.Meters = new(BillingMeters)
		in.Meters.DeepCopyInto(out.Meters)
	}
	if in.Rates != nil {
		out.Rates = new(BillingRates)
		in.Rates.DeepCopyInto(out.Rates)
	}
}

// DeepCopy creates a new BillingPlanSpec instance.
func (in *BillingPlanSpec) DeepCopy() *BillingPlanSpec {
	if in == nil {
		return nil
	}
	out := new(BillingPlanSpec)
	in.DeepCopyInto(out)
	return out
}

// BillingPlanStatus tracks readiness information for a billing plan.
type BillingPlanStatus struct {
	// Conditions surfaces readiness and validation signals.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *BillingPlanStatus) DeepCopyInto(out *BillingPlanStatus) {
	*out = *in
	if in.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(in.Conditions))
		for i := range in.Conditions {
			out.Conditions[i] = *in.Conditions[i].DeepCopy()
		}
	}
}

// DeepCopy creates a new BillingPlanStatus instance.
func (in *BillingPlanStatus) DeepCopy() *BillingPlanStatus {
	if in == nil {
		return nil
	}
	out := new(BillingPlanStatus)
	in.DeepCopyInto(out)
	return out
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="CURRENCY",type=string,JSONPath=`.spec.currency`
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
// BillingPlan defines metering and pricing configuration.
type BillingPlan struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BillingPlanSpec   `json:"spec,omitempty"`
	Status BillingPlanStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// BillingPlanList contains a list of BillingPlan resources.
type BillingPlanList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BillingPlan `json:"items"`
}
