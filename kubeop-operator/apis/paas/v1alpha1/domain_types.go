package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DomainStatusDetails describes DNS and certificate reconciliation details.
type DomainStatusDetails struct {
	// Provider records the DNS provider status message.
	// +optional
	Provider string `json:"provider,omitempty"`

	// Certificate references the certificate status summary.
	// +optional
	Certificate string `json:"certificate,omitempty"`
}

// DomainSpec defines the desired configuration for a DNS domain managed by kubeOP.
type DomainSpec struct {
	// FQDN represents the fully qualified domain name to manage.
	// +kubebuilder:validation:Pattern=`^([a-zA-Z0-9_*-]+\.)+[a-zA-Z]{2,}$`
	FQDN string `json:"fqdn"`

	// TenantRef links the domain to a tenant resource.
	// +kubebuilder:validation:MinLength=1
	TenantRef string `json:"tenantRef"`

	// DNSProviderRef references the credential/secret used for DNS updates.
	// +kubebuilder:validation:MinLength=1
	DNSProviderRef string `json:"dnsProviderRef"`

	// CertificatePolicyRef references the certificate issuance policy for this domain.
	// +optional
	CertificatePolicyRef string `json:"certificatePolicyRef,omitempty"`
}

// DomainStatus captures the observed state of the domain resource.
type DomainStatus struct {
	// DNS summarises DNS reconciliation state.
	// +optional
	DNS *DomainStatusDetails `json:"dns,omitempty"`

	// Certificate summarises certificate issuance state.
	// +optional
	Certificate *DomainStatusDetails `json:"cert,omitempty"`

	// Conditions expresses the readiness and availability of the domain configuration.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="FQDN",type=string,JSONPath=`.spec.fqdn`
// +kubebuilder:printcolumn:name="TENANT",type=string,JSONPath=`.spec.tenantRef`
// +kubebuilder:printcolumn:name="READY",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="AGE",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:validation:XValidation:rule="has(self.metadata.labels['paas.kubeop.io/tenant'])",message="metadata.labels.paas.kubeop.io/tenant is required"
// Domain represents DNS ownership for ingress endpoints managed by kubeOP.
type Domain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DomainSpec   `json:"spec,omitempty"`
	Status DomainStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// DomainList contains a list of Domain resources.
type DomainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Domain `json:"items"`
}
