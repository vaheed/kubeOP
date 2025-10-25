package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AlertRouteType enumerates supported alert destinations.
// +kubebuilder:validation:Enum=pagerduty;webhook;slack
type AlertRouteType string

const (
	// AlertRoutePagerDuty configures PagerDuty destinations.
	AlertRoutePagerDuty AlertRouteType = "pagerduty"
	// AlertRouteWebhook configures generic webhook destinations.
	AlertRouteWebhook AlertRouteType = "webhook"
	// AlertRouteSlack configures Slack webhook destinations.
	AlertRouteSlack AlertRouteType = "slack"
)

// AlertRoute defines a notification target for alerts.
type AlertRoute struct {
	// Name identifies the route for reference by severity rules.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Type declares the destination integration type.
	Type AlertRouteType `json:"type"`

	// Endpoint is the URL or integration key for the route.
	// +kubebuilder:validation:MinLength=1
	Endpoint string `json:"endpoint"`

	// SecretRef optionally references sensitive credentials.
	// +optional
	SecretRef string `json:"secretRef,omitempty"`
}

// AlertSeverityLevel defines severity categories for routing rules.
// +kubebuilder:validation:Enum=info;warning;critical
type AlertSeverityLevel string

// AlertSeverityRule wires severities to routes.
type AlertSeverityRule struct {
	// Severity identifies the severity level for the rule.
	Severity AlertSeverityLevel `json:"severity"`

	// Routes lists route names that should receive the alert.
	// +kubebuilder:validation:MinItems=1
	Routes []string `json:"routes"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *AlertSeverityRule) DeepCopyInto(out *AlertSeverityRule) {
	*out = *in
	if in.Routes != nil {
		out.Routes = append([]string(nil), in.Routes...)
	}
}

// DeepCopy creates a new instance of AlertSeverityRule.
func (in *AlertSeverityRule) DeepCopy() *AlertSeverityRule {
	if in == nil {
		return nil
	}
	out := new(AlertSeverityRule)
	in.DeepCopyInto(out)
	return out
}

// AlertPolicySpec defines routing behaviour for alerts emitted by kubeOP resources.
type AlertPolicySpec struct {
	// MatchLabels selects subjects that should use this policy.
	// +optional
	MatchLabels map[string]string `json:"matchLabels,omitempty"`

	// Routes enumerates notification targets.
	// +kubebuilder:validation:MinItems=1
	Routes []AlertRoute `json:"routes"`

	// SeverityRules maps severities to routes.
	// +kubebuilder:validation:MinItems=1
	SeverityRules []AlertSeverityRule `json:"severityRules"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *AlertPolicySpec) DeepCopyInto(out *AlertPolicySpec) {
	*out = *in
	if in.MatchLabels != nil {
		out.MatchLabels = make(map[string]string, len(in.MatchLabels))
		for k, v := range in.MatchLabels {
			out.MatchLabels[k] = v
		}
	}
	if in.Routes != nil {
		out.Routes = make([]AlertRoute, len(in.Routes))
		copy(out.Routes, in.Routes)
	}
	if in.SeverityRules != nil {
		out.SeverityRules = make([]AlertSeverityRule, len(in.SeverityRules))
		for i := range in.SeverityRules {
			in.SeverityRules[i].DeepCopyInto(&out.SeverityRules[i])
		}
	}
}

// DeepCopy creates a new AlertPolicySpec instance.
func (in *AlertPolicySpec) DeepCopy() *AlertPolicySpec {
	if in == nil {
		return nil
	}
	out := new(AlertPolicySpec)
	in.DeepCopyInto(out)
	return out
}

// AlertPolicyStatus reports reconciliation results for an alert policy.
type AlertPolicyStatus struct {
	// Conditions conveys readiness and configuration health.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// DeepCopyInto copies the receiver, writing into out. in must be non-nil.
func (in *AlertPolicyStatus) DeepCopyInto(out *AlertPolicyStatus) {
	*out = *in
	if in.Conditions != nil {
		out.Conditions = make([]metav1.Condition, len(in.Conditions))
		for i := range in.Conditions {
			out.Conditions[i] = *in.Conditions[i].DeepCopy()
		}
	}
}

// DeepCopy creates a new AlertPolicyStatus instance.
func (in *AlertPolicyStatus) DeepCopy() *AlertPolicyStatus {
	if in == nil {
		return nil
	}
	out := new(AlertPolicyStatus)
	in.DeepCopyInto(out)
	return out
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
// AlertPolicy controls alert delivery for tenants, projects, or apps.
type AlertPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AlertPolicySpec   `json:"spec,omitempty"`
	Status AlertPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// AlertPolicyList contains a list of AlertPolicy resources.
type AlertPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AlertPolicy `json:"items"`
}
