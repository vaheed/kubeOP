package v1alpha1

import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    runtime "k8s.io/apimachinery/pkg/runtime"
    schema "k8s.io/apimachinery/pkg/runtime/schema"
)

// GroupVersion for this API.
var GroupVersion = schema.GroupVersion{Group: "paas.kubeop.io", Version: "v1alpha1"}

// AddToScheme registers types to a scheme.
func AddToScheme(s *runtime.Scheme) error {
    s.AddKnownTypes(GroupVersion,
        &Tenant{}, &TenantList{},
        &Project{}, &ProjectList{},
        &App{}, &AppList{},
        &DNSRecord{}, &DNSRecordList{},
        &Certificate{}, &CertificateList{},
        &Policy{}, &PolicyList{},
        &Registry{}, &RegistryList{},
    )
    metav1.AddToGroupVersion(s, GroupVersion)
    return nil
}

// +k8s:deepcopy-gen=false
// Minimal structs without codegen; rely on runtime.Object interfaces.

type TenantSpec struct {
    Name string `json:"name,omitempty"`
}
type Condition struct {
    Type               string      `json:"type,omitempty"`
    Status             string      `json:"status,omitempty"`
    Reason             string      `json:"reason,omitempty"`
    Message            string      `json:"message,omitempty"`
    LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}
type TenantStatus struct {
    Ready      bool        `json:"ready,omitempty"`
    Conditions []Condition `json:"conditions,omitempty"`
}
type Tenant struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              TenantSpec   `json:"spec,omitempty"`
    Status            TenantStatus `json:"status,omitempty"`
}
func (t *Tenant) DeepCopyObject() runtime.Object { return t }
type TenantList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []Tenant `json:"items"`
}
func (t *TenantList) DeepCopyObject() runtime.Object { return t }

type ProjectSpec struct {
    TenantRef string `json:"tenantRef,omitempty"`
    Name      string `json:"name,omitempty"`
}
type ProjectStatus struct {
    Namespace  string      `json:"namespace,omitempty"`
    Ready      bool        `json:"ready,omitempty"`
    Conditions []Condition `json:"conditions,omitempty"`
}
type Project struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              ProjectSpec   `json:"spec,omitempty"`
    Status            ProjectStatus `json:"status,omitempty"`
}
func (p *Project) DeepCopyObject() runtime.Object { return p }
type ProjectList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []Project `json:"items"`
}
func (p *ProjectList) DeepCopyObject() runtime.Object { return p }

type AppSpec struct {
    Type  string `json:"type,omitempty"`
    Image string `json:"image,omitempty"`
    Host  string `json:"host,omitempty"`
}
type AppStatus struct {
    Ready      bool        `json:"ready,omitempty"`
    Revision   string      `json:"revision,omitempty"`
    Conditions []Condition `json:"conditions,omitempty"`
}
type App struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              AppSpec   `json:"spec,omitempty"`
    Status            AppStatus `json:"status,omitempty"`
}
func (a *App) DeepCopyObject() runtime.Object { return a }
type AppList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []App `json:"items"`
}
func (a *AppList) DeepCopyObject() runtime.Object { return a }

type PolicySpec struct {
    EgressAllowCIDRs []string `json:"egressAllowCIDRs,omitempty"`
}
type Policy struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              PolicySpec `json:"spec,omitempty"`
}
func (p *Policy) DeepCopyObject() runtime.Object { return p }
type PolicyList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []Policy `json:"items"`
}
func (p *PolicyList) DeepCopyObject() runtime.Object { return p }

type RegistrySpec struct {
    Host       string `json:"host,omitempty"`
    Username   string `json:"username,omitempty"`
    PasswordRef string `json:"passwordRef,omitempty"`
}
type Registry struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              RegistrySpec `json:"spec,omitempty"`
}
func (r *Registry) DeepCopyObject() runtime.Object { return r }
type RegistryList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []Registry `json:"items"`
}
func (r *RegistryList) DeepCopyObject() runtime.Object { return r }

type DNSRecordSpec struct {
    Host   string `json:"host,omitempty"`
    Target string `json:"target,omitempty"`
}
type DNSRecordStatus struct {
    Ready   bool   `json:"ready,omitempty"`
    Message string `json:"message,omitempty"`
}
type DNSRecord struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              DNSRecordSpec   `json:"spec,omitempty"`
    Status            DNSRecordStatus `json:"status,omitempty"`
}
func (d *DNSRecord) DeepCopyObject() runtime.Object { return d }
type DNSRecordList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []DNSRecord `json:"items"`
}
func (d *DNSRecordList) DeepCopyObject() runtime.Object { return d }

type CertificateSpec struct {
    Host        string `json:"host,omitempty"`
    DNSRecordRef string `json:"dnsRecordRef,omitempty"`
}
type CertificateStatus struct {
    Ready   bool   `json:"ready,omitempty"`
    Message string `json:"message,omitempty"`
}
type Certificate struct {
    metav1.TypeMeta   `json:",inline"`
    metav1.ObjectMeta `json:"metadata,omitempty"`
    Spec              CertificateSpec   `json:"spec,omitempty"`
    Status            CertificateStatus `json:"status,omitempty"`
}
func (c *Certificate) DeepCopyObject() runtime.Object { return c }
type CertificateList struct {
    metav1.TypeMeta `json:",inline"`
    metav1.ListMeta `json:"metadata,omitempty"`
    Items           []Certificate `json:"items"`
}
func (c *CertificateList) DeepCopyObject() runtime.Object { return c }

