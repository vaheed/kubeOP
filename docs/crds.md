# CRDs

## App
- `json:",inline"`
- `json:"metadata,omitempty"`
- Spec `json:"spec,omitempty"`
- Status `json:"status,omitempty"`

## AppSpec
- Type `json:"type,omitempty"`
- Image `json:"image,omitempty"`
- Host `json:"host,omitempty"`
- Git `json:"git,omitempty"`
- Helm `json:"helm,omitempty"`
- RawManifests `json:"rawManifests,omitempty"`
- Hooks `json:"hooks,omitempty"`

## AppStatus
- Ready `json:"ready,omitempty"`
- Revision `json:"revision,omitempty"`
- Conditions `json:"conditions,omitempty"`

## Certificate
- `json:",inline"`
- `json:"metadata,omitempty"`
- Spec `json:"spec,omitempty"`
- Status `json:"status,omitempty"`

## CertificateSpec
- Host `json:"host,omitempty"`
- DNSRecordRef `json:"dnsRecordRef,omitempty"`

## CertificateStatus
- Ready `json:"ready,omitempty"`
- Message `json:"message,omitempty"`

## DNSRecord
- `json:",inline"`
- `json:"metadata,omitempty"`
- Spec `json:"spec,omitempty"`
- Status `json:"status,omitempty"`

## DNSRecordSpec
- Host `json:"host,omitempty"`
- Target `json:"target,omitempty"`

## DNSRecordStatus
- Ready `json:"ready,omitempty"`
- Message `json:"message,omitempty"`

## PolicySpec
- EgressAllowCIDRs `json:"egressAllowCIDRs,omitempty"`

## Project
- `json:",inline"`
- `json:"metadata,omitempty"`
- Spec `json:"spec,omitempty"`
- Status `json:"status,omitempty"`

## ProjectSpec
- TenantRef `json:"tenantRef,omitempty"`
- Name `json:"name,omitempty"`

## ProjectStatus
- Namespace `json:"namespace,omitempty"`
- Ready `json:"ready,omitempty"`
- Conditions `json:"conditions,omitempty"`

## RegistrySpec
- Host `json:"host,omitempty"`
- Username `json:"username,omitempty"`
- PasswordRef `json:"passwordRef,omitempty"`

## Tenant
- `json:",inline"`
- `json:"metadata,omitempty"`
- Spec `json:"spec,omitempty"`
- Status `json:"status,omitempty"`

## TenantSpec
- Name `json:"name,omitempty"`

## TenantStatus
- Ready `json:"ready,omitempty"`
- Conditions `json:"conditions,omitempty"`
