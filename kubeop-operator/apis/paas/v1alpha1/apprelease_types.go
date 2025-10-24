package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AppReleaseSpec captures rendered artifacts for an application release.
type AppReleaseSpec struct {
	// AppRef references the parent App resource.
	// +kubebuilder:validation:MinLength=1
	AppRef string `json:"appRef"`

	// ResolvedSource details the exact source resolved for this release.
	// +optional
	ResolvedSource string `json:"resolvedSource,omitempty"`

	// Digest stores the OCI or Git digest for the rendered artifacts.
	// +optional
	Digest string `json:"digest,omitempty"`

	// RenderedConfigHash captures a hash of the rendered manifests.
	// +kubebuilder:validation:MinLength=1
	RenderedConfigHash string `json:"renderedConfigHash"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *AppReleaseSpec) DeepCopyInto(out *AppReleaseSpec) {
	*out = *in
}

// DeepCopy creates a new AppReleaseSpec instance.
func (in *AppReleaseSpec) DeepCopy() *AppReleaseSpec {
	if in == nil {
		return nil
	}
	out := new(AppReleaseSpec)
	in.DeepCopyInto(out)
	return out
}

// AppReleaseStatus reports lifecycle details for an AppRelease.
type AppReleaseStatus struct {
	// DeployedAt records the timestamp when the release became active.
	// +optional
	DeployedAt *metav1.Time `json:"deployedAt,omitempty"`

	// RollbackTo optionally references a previous release for rollback.
	// +optional
	RollbackTo string `json:"rollbackTo,omitempty"`

	// Conditions describes readiness for the release.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *AppReleaseStatus) DeepCopyInto(out *AppReleaseStatus) {
	*out = *in
	if in.DeployedAt != nil {
		out.DeployedAt = in.DeployedAt.DeepCopy()
	}
	if in.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(in.Conditions))
		for i := range in.Conditions {
			out.Conditions[i] = *in.Conditions[i].DeepCopy()
		}
	}
}

// DeepCopy creates a new AppReleaseStatus instance.
func (in *AppReleaseStatus) DeepCopy() *AppReleaseStatus {
	if in == nil {
		return nil
	}
	out := new(AppReleaseStatus)
	in.DeepCopyInto(out)
	return out
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="APP",type=string,JSONPath=`.spec.appRef`
// +kubebuilder:printcolumn:name="DIGEST",type=string,JSONPath=`.spec.digest`
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/tenant'])",message="metadata.labels.paas.kubeop.io/tenant is required"
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/project'])",message="metadata.labels.paas.kubeop.io/project is required"
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/app'])",message="metadata.labels.paas.kubeop.io/app is required"
// AppRelease tracks immutable release metadata for an App.
type AppRelease struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppReleaseSpec   `json:"spec,omitempty"`
	Status AppReleaseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// AppReleaseList contains a list of AppRelease resources.
type AppReleaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AppRelease `json:"items"`
}
