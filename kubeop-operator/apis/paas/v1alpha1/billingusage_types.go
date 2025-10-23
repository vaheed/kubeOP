package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BillingSubjectType enumerates metered subjects.
// +kubebuilder:validation:Enum=tenant;project;app
type BillingSubjectType string

// BillingUsageSpec captures a usage snapshot for billing.
type BillingUsageSpec struct {
	// SubjectType identifies the subject of the usage record.
	SubjectType BillingSubjectType `json:"subjectType"`

	// SubjectRef references the subject name.
	// +kubebuilder:validation:MinLength=1
	SubjectRef string `json:"subjectRef"`

	// Window identifies the metering window (hour).
	// +kubebuilder:validation:Pattern=`^\d{4}-\d{2}-\d{2}T\d{2}$`
	Window string `json:"window"`

	// Meters captures point-in-time meter values.
	// +kubebuilder:validation:MinProperties=1
	Meters map[string]string `json:"meters"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *BillingUsageSpec) DeepCopyInto(out *BillingUsageSpec) {
	*out = *in
	if in.Meters != nil {
		out.Meters = make(map[string]string, len(in.Meters))
		for k, v := range in.Meters {
			out.Meters[k] = v
		}
	}
}

// DeepCopy creates a new BillingUsageSpec instance.
func (in *BillingUsageSpec) DeepCopy() *BillingUsageSpec {
	if in == nil {
		return nil
	}
	out := new(BillingUsageSpec)
	in.DeepCopyInto(out)
	return out
}

// BillingUsageStatus conveys processing details for usage records.
type BillingUsageStatus struct {
	// Conditions summarise usage processing status.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *BillingUsageStatus) DeepCopyInto(out *BillingUsageStatus) {
	*out = *in
	if in.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(in.Conditions))
		for i := range in.Conditions {
			out.Conditions[i] = *in.Conditions[i].DeepCopy()
		}
	}
}

// DeepCopy creates a new BillingUsageStatus instance.
func (in *BillingUsageStatus) DeepCopy() *BillingUsageStatus {
	if in == nil {
		return nil
	}
	out := new(BillingUsageStatus)
	in.DeepCopyInto(out)
	return out
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="SUBJECT",type=string,JSONPath=`.spec.subjectRef`
// +kubebuilder:printcolumn:name="WINDOW",type=string,JSONPath=`.spec.window`
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
// BillingUsage captures hourly meter snapshots for billing.
type BillingUsage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BillingUsageSpec   `json:"spec,omitempty"`
	Status BillingUsageStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// BillingUsageList contains a list of BillingUsage resources.
type BillingUsageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BillingUsage `json:"items"`
}

// InvoiceLineItem describes a billed item on an invoice.
type InvoiceLineItem struct {
	// Meter identifies the metered resource.
	// +kubebuilder:validation:MinLength=1
	Meter string `json:"meter"`

	// Quantity records the consumed quantity for the meter.
	// +kubebuilder:validation:MinLength=1
	Quantity string `json:"quantity"`

	// Rate records the unit price.
	// +kubebuilder:validation:MinLength=1
	Rate string `json:"rate"`

	// Total records the line item total.
	// +kubebuilder:validation:MinLength=1
	Total string `json:"total"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *InvoiceLineItem) DeepCopyInto(out *InvoiceLineItem) {
	*out = *in
}

// DeepCopy creates a new InvoiceLineItem instance.
func (in *InvoiceLineItem) DeepCopy() *InvoiceLineItem {
	if in == nil {
		return nil
	}
	out := new(InvoiceLineItem)
	in.DeepCopyInto(out)
	return out
}

// InvoiceSpec defines an invoice aggregating billing usage.
type InvoiceSpec struct {
	// Period identifies the billing period (e.g., 2025-10).
	// +kubebuilder:validation:Pattern=`^\d{4}-\d{2}$`
	Period string `json:"period"`

	// SubjectType identifies the invoiced subject type.
	SubjectType BillingSubjectType `json:"subjectType"`

	// SubjectRef references the invoiced subject name.
	// +kubebuilder:validation:MinLength=1
	SubjectRef string `json:"subjectRef"`

	// LineItems lists metered line items.
	// +kubebuilder:validation:MinItems=1
	LineItems []InvoiceLineItem `json:"lineItems"`

	// Total summarises the invoice total amount.
	// +kubebuilder:validation:MinLength=1
	Total string `json:"total"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *InvoiceSpec) DeepCopyInto(out *InvoiceSpec) {
	*out = *in
	if in.LineItems != nil {
		out.LineItems = make([]InvoiceLineItem, len(in.LineItems))
		for i := range in.LineItems {
			in.LineItems[i].DeepCopyInto(&out.LineItems[i])
		}
	}
}

// DeepCopy creates a new InvoiceSpec instance.
func (in *InvoiceSpec) DeepCopy() *InvoiceSpec {
	if in == nil {
		return nil
	}
	out := new(InvoiceSpec)
	in.DeepCopyInto(out)
	return out
}

// InvoiceStatus reports issuance details for an invoice.
type InvoiceStatus struct {
	// Conditions summarise invoice delivery status.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *InvoiceStatus) DeepCopyInto(out *InvoiceStatus) {
	*out = *in
	if in.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(in.Conditions))
		for i := range in.Conditions {
			out.Conditions[i] = *in.Conditions[i].DeepCopy()
		}
	}
}

// DeepCopy creates a new InvoiceStatus instance.
func (in *InvoiceStatus) DeepCopy() *InvoiceStatus {
	if in == nil {
		return nil
	}
	out := new(InvoiceStatus)
	in.DeepCopyInto(out)
	return out
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="SUBJECT",type=string,JSONPath=`.spec.subjectRef`
// +kubebuilder:printcolumn:name="PERIOD",type=string,JSONPath=`.spec.period`
// +kubebuilder:printcolumn:name="TOTAL",type=string,JSONPath=`.spec.total`
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
// Invoice captures billing totals for a subject.
type Invoice struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   InvoiceSpec   `json:"spec,omitempty"`
	Status InvoiceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// InvoiceList contains a list of Invoice resources.
type InvoiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Invoice `json:"items"`
}
