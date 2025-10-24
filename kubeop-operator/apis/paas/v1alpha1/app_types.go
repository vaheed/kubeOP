package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// AppConditionReady surfaces the overall readiness state for the reconciled workload.
	AppConditionReady = "Ready"

	// AppReadyReasonReconciled indicates that the reconcile loop successfully processed the App.
	AppReadyReasonReconciled = "Reconciled"
)

// AppType enumerates supported deployment mechanisms.
// +kubebuilder:validation:Enum=helmRepo;helmOCI;kustomize;raw;git
type AppType string

const (
	// AppTypeHelmRepo deploys charts from Helm repositories.
	AppTypeHelmRepo AppType = "helmRepo"
	// AppTypeHelmOCI deploys charts from OCI registries.
	AppTypeHelmOCI AppType = "helmOCI"
	// AppTypeKustomize renders kustomize overlays.
	AppTypeKustomize AppType = "kustomize"
	// AppTypeRaw applies raw manifests.
	AppTypeRaw AppType = "raw"
	// AppTypeGit reconciles manifests from Git repositories.
	AppTypeGit AppType = "git"
)

// AppSource references the upstream configuration for an application.
type AppSource struct {
	// URL points at the source repository, registry, or manifest base.
	// +optional
	URL string `json:"url,omitempty"`

	// Chart references a Helm chart name when using chart repositories.
	// +optional
	Chart string `json:"chart,omitempty"`

	// Ref captures a git reference (branch, tag, commit) or OCI digest.
	// +optional
	Ref string `json:"ref,omitempty"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *AppSource) DeepCopyInto(out *AppSource) {
	*out = *in
}

// DeepCopy creates a new AppSource instance.
func (in *AppSource) DeepCopy() *AppSource {
	if in == nil {
		return nil
	}
	out := new(AppSource)
	in.DeepCopyInto(out)
	return out
}

// AppRolloutStrategy enumerates supported rollout strategies.
// +kubebuilder:validation:Enum=rolling;blueGreen;canary
// +kubebuilder:default=rolling
type AppRolloutStrategy string

// AppHealthCheck defines readiness probes for rollouts.
type AppHealthCheck struct {
	// Type identifies the health check protocol.
	// +kubebuilder:validation:Enum=http;tcp
	Type string `json:"type"`

	// Endpoint records the endpoint or port for the health check.
	// +kubebuilder:validation:MinLength=1
	Endpoint string `json:"endpoint"`
}

// AppRolloutSpec configures rollout behaviour for an application.
type AppRolloutSpec struct {
	// Strategy selects the rollout approach for updates.
	// +optional
	Strategy AppRolloutStrategy `json:"strategy,omitempty"`

	// MaxUnavailable defines the number of replicas that can be unavailable during the rollout.
	// +optional
	MaxUnavailable *int32 `json:"maxUnavailable,omitempty"`

	// HealthChecks declares endpoints used to validate rollout success.
	// +optional
	HealthChecks []AppHealthCheck `json:"healthChecks,omitempty"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *AppRolloutSpec) DeepCopyInto(out *AppRolloutSpec) {
	*out = *in
	if in.MaxUnavailable != nil {
		out.MaxUnavailable = new(int32)
		*out.MaxUnavailable = *in.MaxUnavailable
	}
	if in.HealthChecks != nil {
		out.HealthChecks = make([]AppHealthCheck, len(in.HealthChecks))
		copy(out.HealthChecks, in.HealthChecks)
	}
}

// DeepCopy creates a new AppRolloutSpec instance.
func (in *AppRolloutSpec) DeepCopy() *AppRolloutSpec {
	if in == nil {
		return nil
	}
	out := new(AppRolloutSpec)
	in.DeepCopyInto(out)
	return out
}

// AppServiceProfile configures service exposure for the App.
type AppServiceProfile struct {
	// Ports lists service ports exposed by the application.
	// +optional
	Ports []int32 `json:"ports,omitempty"`

	// Internal determines whether the service is internal-only.
	// +optional
	Internal bool `json:"internal,omitempty"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *AppServiceProfile) DeepCopyInto(out *AppServiceProfile) {
	*out = *in
	if in.Ports != nil {
		out.Ports = append([]int32(nil), in.Ports...)
	}
}

// DeepCopy creates a new AppServiceProfile instance.
func (in *AppServiceProfile) DeepCopy() *AppServiceProfile {
	if in == nil {
		return nil
	}
	out := new(AppServiceProfile)
	in.DeepCopyInto(out)
	return out
}

// AppSpec defines the desired state of an application managed by kubeOP.
type AppSpec struct {
	// Type selects the delivery mechanism for the application.
	Type AppType `json:"type"`

	// Source describes the upstream source for manifests or charts.
	// +optional
	Source *AppSource `json:"source,omitempty"`

	// Version pin for the application release.
	// +optional
	Version string `json:"version,omitempty"`

	// VersionRange contains a semantic version range when automatic updates are enabled.
	// +optional
	VersionRange string `json:"semverRange,omitempty"`

	// Image references a container image when direct image deployments are used.
	// +optional
	Image string `json:"image,omitempty"`

	// ValuesRefs references ConfigRef resources providing Helm values.
	// +optional
	ValuesRefs []string `json:"valuesRefs,omitempty"`

	// SecretsRefs references SecretRef resources providing sensitive data.
	// +optional
	SecretsRefs []string `json:"secretsRefs,omitempty"`

	// Replicas declares the desired replica count for workloads rendered from images.
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`

	// Rollout defines rollout behaviour for the application.
	// +optional
	Rollout *AppRolloutSpec `json:"rollout,omitempty"`

	// ServiceProfile defines service exposure defaults.
	// +optional
	ServiceProfile *AppServiceProfile `json:"serviceProfile,omitempty"`

	// IngressRefs references IngressRoute resources consumed by the app.
	// +optional
	IngressRefs []string `json:"ingressRefs,omitempty"`

	// Hosts enumerates ingress hosts for direct image deployments.
	// +optional
	Hosts []string `json:"hosts,omitempty"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *AppSpec) DeepCopyInto(out *AppSpec) {
	*out = *in
	if in.Source != nil {
		out.Source = new(AppSource)
		in.Source.DeepCopyInto(out.Source)
	}
	if in.ValuesRefs != nil {
		out.ValuesRefs = append([]string(nil), in.ValuesRefs...)
	}
	if in.SecretsRefs != nil {
		out.SecretsRefs = append([]string(nil), in.SecretsRefs...)
	}
	if in.Replicas != nil {
		out.Replicas = new(int32)
		*out.Replicas = *in.Replicas
	}
	if in.Rollout != nil {
		out.Rollout = new(AppRolloutSpec)
		in.Rollout.DeepCopyInto(out.Rollout)
	}
	if in.ServiceProfile != nil {
		out.ServiceProfile = new(AppServiceProfile)
		in.ServiceProfile.DeepCopyInto(out.ServiceProfile)
	}
	if in.IngressRefs != nil {
		out.IngressRefs = append([]string(nil), in.IngressRefs...)
	}
	if in.Hosts != nil {
		out.Hosts = append([]string(nil), in.Hosts...)
	}
}

// DeepCopy creates a new AppSpec instance.
func (in *AppSpec) DeepCopy() *AppSpec {
	if in == nil {
		return nil
	}
	out := new(AppSpec)
	in.DeepCopyInto(out)
	return out
}

// AppStatus captures the observed state of an application reconciliation.
type AppStatus struct {
	// Revision stores the rendered revision identifier.
	// +optional
	Revision string `json:"revision,omitempty"`

	// Sync records the reconciliation status summary.
	// +optional
	Sync string `json:"sync,omitempty"`

	// URLs lists externally accessible endpoints associated with the app.
	// +optional
	URLs []string `json:"urls,omitempty"`

	// AvailableReplicas reports the number of pods available for the workload.
	// +optional
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// ObservedGeneration mirrors the metadata generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represents the latest reconcile conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *AppStatus) DeepCopyInto(out *AppStatus) {
	*out = *in
	if in.URLs != nil {
		out.URLs = append([]string(nil), in.URLs...)
	}
	if in.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(in.Conditions))
		for i := range in.Conditions {
			out.Conditions[i] = *in.Conditions[i].DeepCopy()
		}
	}
}

// DeepCopy creates a new AppStatus instance.
func (in *AppStatus) DeepCopy() *AppStatus {
	if in == nil {
		return nil
	}
	out := new(AppStatus)
	in.DeepCopyInto(out)
	return out
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="TYPE",type=string,JSONPath=`.spec.type`
// +kubebuilder:printcolumn:name="REVISION",type=string,JSONPath=`.status.revision`
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/tenant'])",message="metadata.labels.paas.kubeop.io/tenant is required"
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/project'])",message="metadata.labels.paas.kubeop.io/project is required"
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/app'])",message="metadata.labels.paas.kubeop.io/app is required"
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/env'])",message="metadata.labels.paas.kubeop.io/env is required"
// +kubebuilder:validation:XValidation:rule="!has(self.spec.valuesRefs) || size(self.spec.valuesRefs) > 0",message="valuesRefs cannot be empty when specified"
// +kubebuilder:validation:XValidation:rule="!(self.spec.type in ['helmRepo','helmOCI']) || size(self.spec.valuesRefs) > 0",message="valuesRefs required for Helm-based applications"
// App represents a kubeOP managed workload rendered from templates, Helm, Git, or raw manifests.
type App struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AppSpec   `json:"spec,omitempty"`
	Status AppStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// AppList contains a list of App resources.
type AppList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []App `json:"items"`
}
