# Security guide

This document outlines kubeOP's security model, recommended hardening steps, and the responsible disclosure process.

## Threat model

- **Control plane compromise** – kubeOP stores kubeconfigs and can deploy workloads; protect the API host and restrict access.
- **Credential leakage** – JWT secrets, kubeconfig encryption keys, and stored Git/registry credentials must remain confidential.
- **Malicious tenants** – enforce Pod Security Admission, quotas, and RBAC to limit impact within namespaces.
- **Network exposure** – secure ingress with TLS, restrict PostgreSQL access, and monitor outbound connections.

## Authentication & authorization

- Admin APIs require JWTs signed with `ADMIN_JWT_SECRET`. Rotate secrets periodically and store them in secure secret managers.
- `DISABLE_AUTH=true` is intended only for local development.
- Projects inherit RBAC scoped to their namespace. kubeOP provisions ServiceAccounts, Roles, and RoleBindings automatically.
- Maintenance mode (`/v1/admin/maintenance`) prevents mutating operations during upgrades.

## Secrets handling

- kubeconfigs are encrypted at rest using `KCFG_ENCRYPTION_KEY` via `internal/crypto` helpers.
- Git and container registry credentials are stored encrypted and only decrypted when necessary for delivery operations.
- Avoid embedding secrets in app specs; rely on credential stores and Kubernetes Secrets created by kubeOP.

## RBAC

- The API runs with least privilege access to PostgreSQL and the filesystem.
- `kubeop-operator` runs in the `kubeop-system` namespace. Grant cluster-wide permissions only for the CRDs and resources it
  manages (Deployments, Services, Ingresses, Jobs, HPAs).
- When `OPERATOR_LEADER_ELECTION=true`, Kubernetes leases enforce single-writer semantics.

## Audit logging

- Request logs include user, path, correlation ID, and duration.
- Project operations append to `${LOGS_ROOT}/projects/<project-id>/events.jsonl`.
- Use `/v1/projects/{id}/logs` and `/v1/projects/{id}/events` for programmatic retrieval.

## Hardening checklist

- ✅ Terminate TLS with a trusted certificate and enforce HTTPS-only clients.
- ✅ Restrict network access to PostgreSQL and the API port.
- ✅ Enable Pod Security Admission (`POD_SECURITY_LEVEL=restricted`) for sensitive clusters.
- ✅ Configure `PAAS_DOMAIN` and DNS providers using least privilege API tokens.
- ✅ Set `HELM_CHART_ALLOWED_HOSTS` so Helm chart downloads can only reach trusted domains.
- ✅ Use `OPERATOR_IMAGE` pinning rather than `:latest` in production.

## Vulnerability disclosure

- Report suspected vulnerabilities privately to <security@kubeOP.io>.
- Include version information, reproduction steps, and impact assessment.
- Expect acknowledgement within 2 business days and coordinated disclosure timelines agreed with maintainers.
- Public disclosure should occur only after a fix is available or by mutual agreement.
