package v1alpha1

import (
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IngressPath defines route matching for an ingress host.
type IngressPath struct {
        // Path matches the HTTP path.
        // +kubebuilder:validation:Pattern=`^/.*`
        Path string `json:"path"`

        // ServiceRef references the backend service for the path.
        // +kubebuilder:validation:MinLength=1
        ServiceRef string `json:"serviceRef"`
}

// IngressTLSConfig references TLS configuration for an ingress route.
type IngressTLSConfig struct {
        // CertificateRef references a CertificateRequest for the TLS secret.
        // +optional
        CertificateRef string `json:"certRef,omitempty"`

        // CertificatePolicyRef references a certificate issuance policy.
        // +optional
        CertificatePolicyRef string `json:"policyRef,omitempty"`
}

// IngressRouteSpec defines ingress exposure for an application.
type IngressRouteSpec struct {
        // Hosts lists hostnames served by the ingress.
        // +kubebuilder:validation:MinItems=1
        Hosts []string `json:"hosts"`

        // Paths defines route mappings for each host.
        // +kubebuilder:validation:MinItems=1
        Paths []IngressPath `json:"paths"`

        // TLS configures TLS settings for the ingress.
        // +optional
        TLS *IngressTLSConfig `json:"tls,omitempty"`

        // ClassName sets the ingress class name.
        // +optional
        ClassName string `json:"className,omitempty"`
}

// IngressRouteStatus reports observed ingress endpoints.
type IngressRouteStatus struct {
        // Addresses lists the allocated ingress addresses.
        // +optional
        Addresses []string `json:"addresses,omitempty"`

        // Conditions summarise readiness of the ingress route.
        // +optional
        Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="HOSTS",type=integer,JSONPath=`size(.spec.hosts)`
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/tenant'])",message="metadata.labels.paas.kubeop.io/tenant is required"
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/project'])",message="metadata.labels.paas.kubeop.io/project is required"
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/app'])",message="metadata.labels.paas.kubeop.io/app is required"
// IngressRoute models ingress exposure for an application.
type IngressRoute struct {
        metav1.TypeMeta   `json:",inline"`
        metav1.ObjectMeta `json:"metadata,omitempty"`

        Spec   IngressRouteSpec   `json:"spec,omitempty"`
        Status IngressRouteStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// IngressRouteList contains a list of IngressRoute resources.
type IngressRouteList struct {
        metav1.TypeMeta `json:",inline"`
        metav1.ListMeta `json:"metadata,omitempty"`
        Items           []IngressRoute `json:"items"`
}
