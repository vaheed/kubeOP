Roadmap

Overview

- Five phases to reach a solid multi-tenant PaaS, prioritized by impact. Each item implies code, tests (under `testcase/`), and docs updates (including `docs/openapi.yaml`).

Phase 1 — PaaS Core Endpoints (highest impact)

- Apps CRUD+status
  - List: `GET /v1/projects/{id}/apps`, `GET /v1/projects/{id}/apps/{appId}` (status: deployment/rs/pods, URLs)
  - Scale/update: `PATCH /v1/projects/{id}/apps/{appId}/scale`, `PATCH /v1/projects/{id}/apps/{appId}/image`, `POST /v1/projects/{id}/apps/{appId}/rollout/restart`
  - Improve delete: cleanup DNS A records if provisioned
- Secrets & config
  - CRUD for app-scoped ConfigMaps/Secrets: `POST/GET/PATCH/DELETE /v1/projects/{id}/configs|secrets`
  - Attach/detach to apps; document envFrom and selective keys
- TLS and domains
  - Cert-manager integration: issue per-app certs; fields `{tls:true, host:"..."}` generate Ingress+Certificate
  - DNS cleanup on app delete
- Tenant isolation defaults
  - Default deny NetworkPolicies for ingress/egress in both tenancy modes; allow DNS, same-namespace traffic, and ingress namespace selector
  - RBAC: add `events` and `networking.k8s.io/ingresses` to user Role
- Kubeconfig lifecycle
  - User kubeconfig renew: `POST /v1/users/{id}/kubeconfig/renew`
  - Clarify TTL and rotation in docs; tests for renew

Phase 2 — Security & Access Controls

- AuthZ model for tenants (non-admin)
  - Tenant API tokens (PATs) or per-user JWTs limited to their namespace/projects
  - Rate limiting per tenant; audit log of mutating requests
- Policy and registries
  - ImagePullSecrets management and registry allowlist
  - Optional runtime policies (e.g., disallow privileged, enforce seccomp) via validating admission docs
- Quotas and budgets
  - Per-user/project LB quota overrides; pod/resource defaults tuned; expose status endpoint to view usage vs quotas

Phase 3 — Observability & Reliability

- Metrics and tracing
  - Prometheus metrics per API route and per K8s action; histogram latencies
  - Optional OpenTelemetry tracing
- App health and drift
  - `GET /v1/projects/{id}/apps/{appId}/health` (readiness/availability)
  - Background reconcilers or periodic checks to surface drift (missing Service/Ingress)
- Idempotency & retries
  - Make deploy/scale/update idempotent via deterministic keys; safe to retry

Phase 4 — Data Layer & API Ergonomics

- Database
  - Partial indexes on `deleted_at IS NULL` for users/projects/apps
  - Replace naive JSON helpers with proper JSONB handling (marshal/unmarshal) in store layer
- API consistency
  - Pagination and filters on all list endpoints (limit/offset, filter by cluster, name)
  - Standard error schema; tighten enums (ServiceType, Protocol) in OpenAPI
  - ReDoc HTML publish of `docs/openapi.yaml` with CI
- DX improvements
  - CLI or thin SDK; better examples; Postman/Insomnia collection

Phase 5 — Advanced PaaS Features

- Build from Git
  - Buildpacks/Kaniko integration to build container from repo; manage image cache and provenance
- Deploy strategies & autoscaling
  - HPA opt-in; simple canary/blue-green via extra Service/Ingress routes
- Multi-cluster scheduling
  - Policy-driven placement (labels, capacity), project affinity/anti-affinity across clusters
- SSO & orgs (optional)
  - SSO integration (OIDC) for admin; organization/team model for grouping users and projects

Dependencies and Notes

- Network policies require label configuration documented in `docs/ISOLATION.md`.
- Cert-manager and DNS providers (Cloudflare/PowerDNS) must be configured via ENV; add docs and negative test paths.
- For any API changes, update `docs/openapi.yaml`, `docs/API_REFERENCE.md`, README pointers, and add unit tests under `testcase/` per AGENTS.md.
