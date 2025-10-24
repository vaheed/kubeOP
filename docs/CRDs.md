# kubeOP platform CRDs

The kubeOP operator exposes a suite of custom resources under the `paas.kubeop.io/v1alpha1` API group. Every resource follows Kubernetes API conventions (status subresources, CEL validations, and printer columns). Use this guide to map high-level responsibilities to the relevant spec and status fields.

## Cluster-scoped resources

### Tenant (`Tenant`)
- **Spec**: `displayName`, immutable `billingAccountRef`, optional `policyRefs[]`, quota soft/hard limits (`cpu`, `memory`, `storage`, `objects`).
- **Status**: `usage` reports live CPU, memory, storage, egress, and load-balancer hour consumption aggregated by the Usage Writer controller; `conditions` follow kstatus (`Ready`, `Reconciling`, `Degraded`).
- **Notes**: Enforces the `paas.kubeop.io/tenant` label on metadata and carries clean-up finalisers.

### Domain (`Domain`)
- **Spec**: `fqdn`, `tenantRef`, `dnsProviderRef`, optional `certificatePolicyRef` for TLS automation.
- **Status**: `dns` (provider state), `cert` (issuer state), `conditions` for availability.

### RegistryCredential (`RegistryCredential`)
- **Spec**: `type` (`dockerHub`, `ecr`, `gcr`, `harbor`), `secretRef`, `tenantRef`.
- **Status**: `conditions` capture credential validity.

### AlertPolicy (`AlertPolicy`)
- **Spec**: `routes[]` (PagerDuty, webhook, Slack targets), `matchLabels`, severity rules for incident routing.
- **Status**: `conditions` summarise policy deployment.

### BillingPlan (`BillingPlan`)
- **Spec**: `meters` (`cpu`, `mem`, `storage`, `egress`, `lbHours`, `ips`, `objects`), `rates` per meter, `currency`.
- **Status**: `conditions` for publication state.

### RuntimeClassProfile (`RuntimeClassProfile`)
- **Spec**: `runtimeClassName`, optional `tolerations`, `nodeSelector`, `securityPreset` to align workloads with PSP/PSA levels.
- **Status**: `conditions` indicate whether the runtime is usable.

## Namespace-scoped resources

### Project (`Project`)
- **Spec**: `tenantRef`, `purpose`, `environment` (`dev`, `stage`, `prod`), `namespaceName`, optional `quotas` (`ResourceQuota`, `LimitRange`), `psapreset`, `networkPolicyProfileRef`.
- **Status**: `syncNs` boolean, aggregated `usage` meters (CPU, memory, storage, egress, load balancer hours), plus `conditions`.

### App (`App`)
- **Spec**: `type` (`helmRepo`, `helmOCI`, `kustomize`, `raw`, `git`), `source` (URL/chart/ref), `version` or semver range, direct `image` deployments with optional `replicas`/`hosts`, `valuesRefs[]`, `secretsRefs[]`, `rollout` strategy/health checks, `serviceProfile`, `ingressRefs[]`.
- **Status**: `revision`, `sync`, exposed `urls[]`, `availableReplicas`, `observedGeneration`, `conditions` reflecting deploy health.

### AppRelease (`AppRelease`)
- **Spec**: `appRef`, `resolvedSource`, `digest`, `renderedConfigHash`.
- **Status**: `deployedAt`, optional `rollbackTo` reference.

### ConfigRef (`ConfigRef`)
- **Spec**: `data` (inline key/value or `configMapRef`), `mount`/`vars` scope for consumers.
- **Status**: `conditions` for availability.

### SecretRef (`SecretRef`)
- **Spec**: Mirrored structure to `ConfigRef`, but targets secret material with `secretRef` or inline values.
- **Status**: `conditions` for readiness.

### IngressRoute (`IngressRoute`)
- **Spec**: `hosts[]`, `paths[]`, `serviceRef`, optional `tls` (`certRef` or `policyRef`), `className`.
- **Status**: `addresses[]`, `conditions` for ingress health.

### CertificateRequest (`CertificateRequest`)
- **Spec**: `dnsNames[]`, `issuerRef` (`ACME` or `CA`), `dns01`/`http01` solver configuration.
- **Status**: `notBefore`, `notAfter`, `conditions` covering issuance.

### Job (`Job`)
- **Spec**: `template` (`PodSpec`), `backoff`, optional `ttlSecondsAfterFinished`, optional `schedule` for cron execution.
- **Status**: Last run timestamps, completion state, `conditions`.

### DatabaseInstance (`DatabaseInstance`)
- **Spec**: `engine` (`pg`, `mysql`), `plan` (size, IOPS), `backupPolicyRef`, `connSecretRef`.
- **Status**: `endpoint`, `conditions`.

### CacheInstance (`CacheInstance`)
- **Spec**: `engine` (`redis`, `memcached`), `plan` sizing, optional `connSecretRef`.
- **Status**: `endpoint`, `conditions`.

### QueueInstance (`QueueInstance`)
- **Spec**: `engine` (`rabbitmq`, `sqs-compat`), `plan`, connection secret reference.
- **Status**: `endpoint`, `conditions`.

### Bucket (`Bucket`)
- **Spec**: `provider` (`minio`, `s3`), `versioning`, `lifecycle` rules, optional `policyRefs[]`.
- **Status**: `conditions` with observed provider state.

### BucketPolicy (`BucketPolicy`)
- **Spec**: `statements[]` (effect, principals, actions, resources, condition blocks).
- **Status**: `conditions` showing application status.

### ServiceBinding (`ServiceBinding`)
- **Spec**: `consumerRef` (app/ServiceAccount), `providerRef` (database/cache/queue/bucket), `injectAs` (`env`, `file`, `secret`).
- **Status**: `conditions`, binding secret details.

### NetworkPolicyProfile (`NetworkPolicyProfile`)
- **Spec**: Named `presets` (`deny-all`, `web-only`, `db-isolate`), explicit ingress/egress rules.
- **Status**: `conditions` indicating propagation to namespaces.

### MetricQuota (`MetricQuota`)
- **Spec**: `targets` (`app`, `project`, `tenant`), limit map for `cpu`, `mem`, `egress`, `lbHours`.
- **Status**: `current` usage snapshot, `conditions`.

### BillingUsage (`BillingUsage`)
- **Spec**: `subjectRef` (`tenant`, `project`, `app`), `window` (hour granularity), `meters` snapshot populated by the Usage Writer controller (`cpu`, `memory`, `storage`, `egress`, `lbHours`).
- **Status**: `conditions` reflect the most recent aggregation attempt.

### Invoice (`Invoice`)
- **Spec**: Billing `period`, `lineItems[]`, `totals` (subtotal, taxes, grand total), `currency`.
- **Status**: `conditions`, emission metadata.

## Cross-cutting conventions

- **Labels**: `paas.kubeop.io/tenant`, `paas.kubeop.io/project`, `paas.kubeop.io/app`, and `paas.kubeop.io/env` are required where applicable. Builders and webhooks enforce these labels to keep ownership queries fast.
- **Status**: All controllers publish `conditions` adhering to the kstatus contract (types include `Ready`, `Reconciling`, `Degraded`), enabling `kubectl get` printer columns (`TENANT`, `PROJECT`, `READY`, `AGE`).
- **Finalizers**: Tenant and Project resources hold cleanup finalisers so the bootstrap CLI and reconciler can remove dependent namespaces and child objects safely.

Combine this document with the generated CRDs under `kubeop-operator/kustomize/bases/crds/` for complete OpenAPI schemas and CEL expressions.
