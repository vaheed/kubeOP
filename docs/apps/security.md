# App delivery safety

kubeOP enforces guardrails around release metadata and upstream sources so tenant workloads remain reproducible and auditable.

## AppRelease immutability

`AppRelease` objects capture the exact artifact digest, rendered manifest hash, and resolved version for an application rollout. Once
created, the spec is immutable—only `.status` fields may change. Mutation requests that touch any field under `.spec` are rejected by
the admission webhook with an `Invalid` error. Immutable release metadata prevents tampering after a deployment is recorded and keeps
rollback history trustworthy.

Validation now ensures:

- `spec.version` must be a semantic version (`MAJOR.MINOR.PATCH` with optional pre-release and build qualifiers).
- `spec.digest` must follow `algorithm:hex` format (for example, `sha256:...`).
- `spec.renderedConfigHash` must be a hexadecimal digest.

## Registry and repository allowlists

Tenant namespaces can opt into registry governance by creating a ConfigMap named `registry-policy`. The webhook reads the ConfigMap on
app creation and update to ensure container images and Helm/OCI sources resolve to approved registries and repositories.

The ConfigMap accepts newline- or comma-separated lists:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: registry-policy
  namespace: team-a
  labels:
    paas.kubeop.io/tenant: tenant-a
    paas.kubeop.io/project: proj
    paas.kubeop.io/app: app
data:
  allowedRegistries: |
    ghcr.io
    quay.io
  allowedRepositories: |
    ghcr.io/platform/allowed*
    quay.io/team/applications
  requireCosign: "true"
```

- `allowedRegistries` limits the registry hosts (`ghcr.io`, `quay.io`). Wildcards in the form of `*.example.com` are supported.
- `allowedRepositories` constrains fully qualified repositories. A trailing `*` matches a prefix (for example `ghcr.io/team/*`).
- `requireCosign` (optional) flags that signatures must be enforced downstream. The webhook surfaces an admission warning when the
  flag is present so operators can wire cosign verification into the delivery pipeline.

An app is rejected when:

- `spec.image` references an unapproved registry or repository.
- `spec.source.url` points at a Helm repo or OCI chart outside the allowlist.

Missing ConfigMaps mean no registry restrictions for that namespace, preserving backwards compatibility until platform teams roll out
policies.

## Operator expectations

- Store the registry policy ConfigMap alongside the tenant's app namespace.
- Keep ConfigMap labels aligned with the tenant/project/app labels enforced by the webhook.
- Update the allowlists before introducing new registries or repositories; otherwise deployments will be blocked during admission.
