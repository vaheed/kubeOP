package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BucketProvider enumerates supported object storage providers.
// +kubebuilder:validation:Enum=minio;s3
type BucketProvider string

// BucketLifecycleRule defines lifecycle transitions for bucket objects.
type BucketLifecycleRule struct {
	// ID uniquely identifies the lifecycle rule.
	// +kubebuilder:validation:MinLength=1
	ID string `json:"id"`

	// Prefix targets objects with the given prefix.
	// +optional
	Prefix string `json:"prefix,omitempty"`

	// ExpireDays deletes objects after the specified number of days.
	// +optional
	ExpireDays int32 `json:"expireDays,omitempty"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *BucketLifecycleRule) DeepCopyInto(out *BucketLifecycleRule) {
	*out = *in
}

// DeepCopy creates a new BucketLifecycleRule instance.
func (in *BucketLifecycleRule) DeepCopy() *BucketLifecycleRule {
	if in == nil {
		return nil
	}
	out := new(BucketLifecycleRule)
	in.DeepCopyInto(out)
	return out
}

// BucketSpec defines bucket provisioning configuration.
type BucketSpec struct {
	// Provider selects the object storage backend.
	Provider BucketProvider `json:"provider"`

	// Versioning toggles object versioning.
	// +optional
	Versioning bool `json:"versioning,omitempty"`

	// Lifecycle defines lifecycle policies applied to bucket objects.
	// +optional
	Lifecycle []BucketLifecycleRule `json:"lifecycle,omitempty"`

	// PolicyRefs references BucketPolicy resources applied to the bucket.
	// +optional
	PolicyRefs []string `json:"policyRefs,omitempty"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *BucketSpec) DeepCopyInto(out *BucketSpec) {
	*out = *in
	if in.Lifecycle != nil {
		out.Lifecycle = make([]BucketLifecycleRule, len(in.Lifecycle))
		for i := range in.Lifecycle {
			in.Lifecycle[i].DeepCopyInto(&out.Lifecycle[i])
		}
	}
	if in.PolicyRefs != nil {
		out.PolicyRefs = append([]string(nil), in.PolicyRefs...)
	}
}

// DeepCopy creates a new BucketSpec instance.
func (in *BucketSpec) DeepCopy() *BucketSpec {
	if in == nil {
		return nil
	}
	out := new(BucketSpec)
	in.DeepCopyInto(out)
	return out
}

// BucketStatus reports observed state for a bucket.
type BucketStatus struct {
	// Conditions summarise provisioning status.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *BucketStatus) DeepCopyInto(out *BucketStatus) {
	*out = *in
	if in.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(in.Conditions))
		for i := range in.Conditions {
			out.Conditions[i] = *in.Conditions[i].DeepCopy()
		}
	}
}

// DeepCopy creates a new BucketStatus instance.
func (in *BucketStatus) DeepCopy() *BucketStatus {
	if in == nil {
		return nil
	}
	out := new(BucketStatus)
	in.DeepCopyInto(out)
	return out
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="PROVIDER",type=string,JSONPath=`.spec.provider`
// +kubebuilder:printcolumn:name="VERSIONING",type=boolean,JSONPath=`.spec.versioning`
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/tenant'])",message="metadata.labels.paas.kubeop.io/tenant is required"
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/project'])",message="metadata.labels.paas.kubeop.io/project is required"
// Bucket provisions object storage for workloads.
type Bucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BucketSpec   `json:"spec,omitempty"`
	Status BucketStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// BucketList contains a list of Bucket resources.
type BucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Bucket `json:"items"`
}

// BucketPolicyStatement defines permissions for object storage consumers.
type BucketPolicyStatement struct {
	// Effect determines whether access is allowed or denied.
	// +kubebuilder:validation:Enum=Allow;Deny
	Effect string `json:"effect"`

	// Actions lists allowed or denied actions.
	// +kubebuilder:validation:MinItems=1
	Actions []string `json:"actions"`

	// Principals lists subjects granted permissions.
	// +kubebuilder:validation:MinItems=1
	Principals []string `json:"principals"`

	// Resources lists ARN-style resource identifiers.
	// +optional
	Resources []string `json:"resources,omitempty"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *BucketPolicyStatement) DeepCopyInto(out *BucketPolicyStatement) {
	*out = *in
	if in.Actions != nil {
		out.Actions = append([]string(nil), in.Actions...)
	}
	if in.Principals != nil {
		out.Principals = append([]string(nil), in.Principals...)
	}
	if in.Resources != nil {
		out.Resources = append([]string(nil), in.Resources...)
	}
}

// DeepCopy creates a new BucketPolicyStatement instance.
func (in *BucketPolicyStatement) DeepCopy() *BucketPolicyStatement {
	if in == nil {
		return nil
	}
	out := new(BucketPolicyStatement)
	in.DeepCopyInto(out)
	return out
}

// BucketPolicySpec defines reusable bucket policy statements.
type BucketPolicySpec struct {
	// Statements enumerates IAM-style policy statements.
	// +kubebuilder:validation:MinItems=1
	Statements []BucketPolicyStatement `json:"statements"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *BucketPolicySpec) DeepCopyInto(out *BucketPolicySpec) {
	*out = *in
	if in.Statements != nil {
		out.Statements = make([]BucketPolicyStatement, len(in.Statements))
		for i := range in.Statements {
			in.Statements[i].DeepCopyInto(&out.Statements[i])
		}
	}
}

// DeepCopy creates a new BucketPolicySpec instance.
func (in *BucketPolicySpec) DeepCopy() *BucketPolicySpec {
	if in == nil {
		return nil
	}
	out := new(BucketPolicySpec)
	in.DeepCopyInto(out)
	return out
}

// BucketPolicyStatus reports validation results for a bucket policy.
type BucketPolicyStatus struct {
	// Conditions summarise policy validation status.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *BucketPolicyStatus) DeepCopyInto(out *BucketPolicyStatus) {
	*out = *in
	if in.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(in.Conditions))
		for i := range in.Conditions {
			out.Conditions[i] = *in.Conditions[i].DeepCopy()
		}
	}
}

// DeepCopy creates a new BucketPolicyStatus instance.
func (in *BucketPolicyStatus) DeepCopy() *BucketPolicyStatus {
	if in == nil {
		return nil
	}
	out := new(BucketPolicyStatus)
	in.DeepCopyInto(out)
	return out
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="STATEMENTS",type=integer,JSONPath=`size(.spec.statements)`
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/tenant'])",message="metadata.labels.paas.kubeop.io/tenant is required"
// BucketPolicy defines reusable policies for buckets.
type BucketPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BucketPolicySpec   `json:"spec,omitempty"`
	Status BucketPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// BucketPolicyList contains a list of BucketPolicy resources.
type BucketPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BucketPolicy `json:"items"`
}
