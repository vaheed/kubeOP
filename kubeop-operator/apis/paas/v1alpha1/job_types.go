package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// JobSpec defines a templated job or cron schedule managed by kubeOP.
type JobSpec struct {
	// Template describes the pod template to run for the job.
	Template corev1.PodTemplateSpec `json:"template"`

	// BackoffLimit configures retry attempts for failed jobs.
	// +optional
	BackoffLimit *int32 `json:"backoffLimit,omitempty"`

	// TTLSecondsAfterFinished defines when completed jobs are cleaned up.
	// +optional
	TTLSecondsAfterFinished *int32 `json:"ttlSecondsAfterFinished,omitempty"`

	// Schedule optionally defines a cron expression to run on a schedule.
	// +optional
	Schedule string `json:"schedule,omitempty"`
}

// JobStatus reports execution details for a job.
type JobStatus struct {
	// Active tracks the number of actively running pods.
	// +optional
	Active int32 `json:"active,omitempty"`

	// Succeeded counts completed runs.
	// +optional
	Succeeded int32 `json:"succeeded,omitempty"`

	// Failed counts failed runs.
	// +optional
	Failed int32 `json:"failed,omitempty"`

	// StartTime records the time when the job started.
	// +optional
	StartTime *metav1.Time `json:"startTime,omitempty"`

	// CompletionTime records when the job completed.
	// +optional
	CompletionTime *metav1.Time `json:"completionTime,omitempty"`

	// Conditions summarise job health.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="SCHEDULE",type=string,JSONPath=`.spec.schedule`
// +kubebuilder:printcolumn:name="ACTIVE",type=integer,JSONPath=`.status.active`
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/tenant'])",message="metadata.labels.paas.kubeop.io/tenant is required"
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/project'])",message="metadata.labels.paas.kubeop.io/project is required"
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/app'])",message="metadata.labels.paas.kubeop.io/app is required"
// Job represents a managed Job or CronJob workload.
type Job struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   JobSpec   `json:"spec,omitempty"`
	Status JobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// JobList contains a list of Job resources.
type JobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Job `json:"items"`
}
