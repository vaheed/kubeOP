package v1alpha1

import (
        metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CertificateIssuerType enumerates supported certificate issuers.
// +kubebuilder:validation:Enum=ACME;CA
type CertificateIssuerType string

// CertificateRequestIssuer references the issuer for a certificate request.
type CertificateRequestIssuer struct {
        // Type identifies the issuer type (ACME or CA).
        Type CertificateIssuerType `json:"type"`

        // Name references the issuer resource name.
        // +kubebuilder:validation:MinLength=1
        Name string `json:"name"`
}

// DNS01SolverConfig defines DNS-01 challenge configuration.
type DNS01SolverConfig struct {
        // ProviderRef references the DNS provider credential to use.
        // +kubebuilder:validation:MinLength=1
        ProviderRef string `json:"providerRef"`
}

// HTTP01SolverConfig defines HTTP-01 challenge configuration.
type HTTP01SolverConfig struct {
        // ServiceRef references the service exposing challenge responses.
        // +kubebuilder:validation:MinLength=1
        ServiceRef string `json:"serviceRef"`
}

// CertificateRequestSpec declares desired certificate issuance configuration.
type CertificateRequestSpec struct {
        // DNSNames lists DNS names to include in the certificate.
        // +kubebuilder:validation:MinItems=1
        DNSNames []string `json:"dnsNames"`

        // IssuerRef references the issuer handling the request.
        IssuerRef CertificateRequestIssuer `json:"issuerRef"`

        // DNS01 configures DNS-01 challenge solving.
        // +optional
        DNS01 *DNS01SolverConfig `json:"dns01,omitempty"`

        // HTTP01 configures HTTP-01 challenge solving.
        // +optional
        HTTP01 *HTTP01SolverConfig `json:"http01,omitempty"`
}

// CertificateRequestStatus conveys issuance details for a certificate.
type CertificateRequestStatus struct {
        // NotBefore records the certificate validity start time.
        // +optional
        NotBefore *metav1.Time `json:"notBefore,omitempty"`

        // NotAfter records the certificate expiry time.
        // +optional
        NotAfter *metav1.Time `json:"notAfter,omitempty"`

        // Conditions summarise issuance status.
        // +optional
        Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="ISSUER",type=string,JSONPath=`.spec.issuerRef.name`
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/tenant'])",message="metadata.labels.paas.kubeop.io/tenant is required"
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/project'])",message="metadata.labels.paas.kubeop.io/project is required"
// CertificateRequest models certificate issuance for ingress routes.
type CertificateRequest struct {
        metav1.TypeMeta   `json:",inline"`
        metav1.ObjectMeta `json:"metadata,omitempty"`

        Spec   CertificateRequestSpec   `json:"spec,omitempty"`
        Status CertificateRequestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// CertificateRequestList contains a list of CertificateRequest resources.
type CertificateRequestList struct {
        metav1.TypeMeta `json:",inline"`
        metav1.ListMeta `json:"metadata,omitempty"`
        Items           []CertificateRequest `json:"items"`
}
