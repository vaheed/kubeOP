# Security guide

This document explains kubeOP’s security posture, threats, and hardening guidance. Pair it with [ENVIRONMENT](ENVIRONMENT.md) to
configure secrets and toggles correctly.

## Threat model

- **Control plane compromise** – An attacker gains access to the kubeOP API or database. Mitigate with strong secrets, network
  segmentation, and audit logging.
- **Managed cluster compromise** – A cluster kubeconfig is exfiltrated. Mitigate by rotating kubeconfigs, enforcing RBAC in
  managed clusters, and limiting kubeOP’s cluster-scoped permissions.
- **Supply-chain risk** – Unverified artifacts enter the delivery pipeline. Mitigate with SBOM validation (`/v1/projects/{id}/apps/{appId}/delivery`)
  and release history checks.
- **Event spoofing** – Malicious actors send forged events. Only enable `EVENT_BRIDGE_ENABLED` when the ingest endpoint is behind
  authenticated infrastructure.

## Authentication and authorisation

- kubeOP uses HMAC JWTs signed with `ADMIN_JWT_SECRET`. Only tokens with `{"role":"admin"}` are accepted.
- Set `DISABLE_AUTH=false` in every environment except isolated testing.
- JWT claims may include `sub`, `user_id`, or `email`. These values appear in audit logs for traceability.
- Use reverse proxies or identity providers to front kubeOP if you need user-level RBAC; the built-in API assumes administrative
  callers.

## Secret management

| Secret | Purpose | Rotation guidance |
| --- | --- | --- |
| `ADMIN_JWT_SECRET` | Signs administrator JWTs. | Rotate quarterly or after staff changes. Requires reissuing tokens. |
| `KCFG_ENCRYPTION_KEY` | Encrypts stored kubeconfigs (AES-GCM). | Rotate annually with a brief maintenance window to re-encrypt. |
| Database credentials | PostgreSQL access. | Rotate according to organisational policy; update `DATABASE_URL` and Kubernetes Secrets. |
| `GIT_WEBHOOK_SECRET` | Validates Git webhook payloads. | Rotate when rotating repository webhooks. |

Store secrets in your preferred secret manager (Kubernetes Secrets, HashiCorp Vault, cloud secret stores). Avoid committing
secrets to Git.

## Network boundaries

- Restrict kubeOP’s inbound access to trusted operators or CI systems. Use mutual TLS or API gateways where possible.
- Allow outbound access from kubeOP to managed cluster API servers, PostgreSQL, and optional DNS providers.
- Disable `ALLOW_INSECURE_HTTP` in production. Always set `KUBEOP_BASE_URL` to an HTTPS endpoint fronted by TLS termination.

## RBAC in managed clusters

- The `kubeop-operator` should run with namespaced permissions scoped to the tenant namespace plus required cluster-level access
  for CRDs and leader election.
- Enable `OPERATOR_LEADER_ELECTION=true` when running multiple operator replicas.
- Audit generated manifests for the correct `kubeop.*` labels and ensure quota/LimitRange policies match organisational standards.

## Handling vulnerability reports

- Report vulnerabilities to [security@kubeOP.io](mailto:security@kubeOP.io). See [SUPPORT](https://github.com/vaheed/kubeOP/blob/main/SUPPORT.md) for response timelines.
- Maintainers will coordinate disclosure and publish fixes along with CHANGELOG entries.
- kubeOP does not issue CVEs automatically. The team will assign identifiers for high/critical issues affecting released builds.

## Secure delivery practices

- Validate SBOM fingerprints and manifest digests via `/v1/projects/{id}/apps/{appId}/delivery` before promoting releases.
- Require Git and registry credentials to use least-privilege scopes (`project` or `tenant`) when possible.
- Disable `ALLOW_GIT_FILE_PROTOCOL` outside local testing to prevent repository escape vectors.
- Keep dependencies updated via `go mod tidy` and review `go.sum` changes for suspicious additions.

## Incident response

1. Enable maintenance mode to freeze mutating requests.
2. Rotate JWT and kubeconfig encryption secrets.
3. Audit `/v1/projects/{id}/apps/{appId}/releases` for unexpected rollouts.
4. Review logs using request IDs to trace suspicious actions.
5. Redeploy the API and operator with clean images.
6. Coordinate disclosure with reporters if the incident is user-visible.

## Compliance checklist

- [ ] Secrets stored outside Git and rotated per policy.
- [ ] HTTPS enforced for external endpoints.
- [ ] PostgreSQL backups tested.
- [ ] Operator manifests managed declaratively (GitOps/Helm).
- [ ] Maintenance mode procedures documented for upgrades and incidents.
