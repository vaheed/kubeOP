Roadmap

Overview

- Five phases to reach a solid multi-tenant PaaS, prioritized by impact. Each item implies code, tests (under `testcase/`), and docs updates (including `docs/openapi.yaml`).
- Immediate next steps highlight the most actionable backlog based on the current code layout (service layer, scheduler helper, manifest builders, CI). Treat them as the default sprint plan.

Immediate Next Steps (0-2 sprints)

1. **Scheduler observability**
   - Expose `/metrics` counters for `cluster_health_ticks_total`, `cluster_health_errors_total`, and duration histograms via the new scheduler helper.
   - Add structured log sampling or log level controls for noisy clusters; document expectations in `docs/METRICS.md` and `docs/OPERATIONS.md`.
2. **Manifest drift detection**
   - Use the shared manifest builders inside reconciliation logic that verifies tenant namespaces match desired policies.
   - Add tests stubbing controller-runtime fake clients to detect missing NetworkPolicies/RoleBindings.
3. **CI hardening**
   - Enforce `gofmt` (now wired) and extend linting with `staticcheck` or `golangci-lint` presets for API packages.
   - Upload coverage artifacts and build metadata JSON for traceability; update `.github/workflows/ci.yml` accordingly.
4. **Docs & runbooks**
   - Fill in the drafted CONTRIBUTING, OPERATIONS, and SECURITY docs with org-specific policies once decisions land.
   - Publish the documentation plan and keep the doc set table updated in PR templates.


Phase 1 — PaaS Core Endpoints (highest impact)

1. **Apps CRUD + status**
   - Implement listing (`GET /v1/projects/{id}/apps`, `GET /v1/projects/{id}/apps/{appId}`) with deployment/service/pod summaries.
   - Support scaling + image updates (`PATCH /scale`, `PATCH /image`) and rollout restarts.
   - Ensure delete cleans DNS A records if the app provisioned them.
   - Extend `internal/service/apps.go` with shared helpers and cover edge cases in `testcase/service_status_test.go` (multiple Services/Ingresses, empty selectors).
   - Document request/response schemas in `docs/API_REFERENCE.md` and keep `docs/openapi.yaml` aligned.
2. **Secrets & Config**
   - Deliver CRUD for app-scoped ConfigMaps/Secrets (`POST/GET/PATCH/DELETE`).
   - Provide attach/detach endpoints with documentation for envFrom vs selective keys.
   - Harden JSON handling via `service.EncodeQuotaOverrides`/`DecodeQuotaOverrides` and add regression tests for escaping/whitespace.
3. **TLS and domains**
   - Integrate cert-manager so `{tls:true, host:"..."}` creates Ingress + Certificate.
   - Remove DNS records automatically when apps are deleted.
   - Expand `internal/dns` providers with retry/backoff logging and add fixtures under `testcase/dns_provider_test.go` for new providers.
4. **Tenant isolation defaults**
   - Ship deny-by-default NetworkPolicies while allowing DNS, intra-namespace traffic, and ingress namespace selectors.
   - Extend tenant RBAC to include `events` and `networking.k8s.io/ingresses`.
   - Capture policies in `docs/ISOLATION.md` with namespace/label tables that mirror the defaults in `internal/service/service.go`.
5. **Kubeconfig lifecycle**
   - Add kubeconfig renew endpoint (`POST /v1/users/{id}/kubeconfig/renew`).
   - Document rotation/TTL expectations and add tests covering the renewal flow.
   - Keep kubeconfig labels readable via `service.ResolveUserLabel` and verify renewals in `testcase` maintain the same label for stable contexts.

Phase 2 — Security & Access Controls

- AuthZ model for tenants (non-admin)
  - Tenant API tokens (PATs) or per-user JWTs limited to their namespace/projects
  - Rate limiting per tenant; audit log of mutating requests
  - Introduce request logging middleware updates that emit status codes and correlate with tenant IDs for later analysis.
- Policy and registries
  - ImagePullSecrets management and registry allowlist
  - Optional runtime policies (e.g., disallow privileged, enforce seccomp) via validating admission docs
- Quotas and budgets
  - Per-user/project LB quota overrides; pod/resource defaults tuned; expose status endpoint to view usage vs quotas

Phase 3 — Observability & Reliability

- Metrics and tracing
  - Prometheus metrics per API route and per K8s action; histogram latencies
  - Optional OpenTelemetry tracing
  - Emit scheduler lifecycle metrics (ticks, failures) leveraging the new `ClusterHealthScheduler` helper and document scrape setup in `docs/METRICS.md`.
- App health and drift
  - `GET /v1/projects/{id}/apps/{appId}/health` (readiness/availability)
  - Background reconcilers or periodic checks to surface drift (missing Service/Ingress)
- Idempotency & retries
  - Make deploy/scale/update idempotent via deterministic keys; safe to retry

Phase 4 — Data Layer & API Ergonomics

- Database
  - Partial indexes on `deleted_at IS NULL` for users/projects/apps
  - Replace naive JSON helpers with proper JSONB handling (marshal/unmarshal) in store layer
  - Audit quota override persistence to ensure `service.EncodeQuotaOverrides` integrates cleanly with JSONB migrations.
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

Open Questions

1. What minimal Service Level Objectives should the control plane commit to (e.g., health tick latency, API availability) and how will they be enforced/alerted?
2. Should tenant namespace manifests eventually move to a declarative GitOps flow (e.g., Argo CD) instead of controller-runtime patches for better drift detection?
3. How many clusters are expected in production, and do we need sharding or work queueing for the scheduler to keep tick durations bounded under load?
4. Which secrets management approach (external vault vs. Kubernetes secrets) is acceptable for kubeconfig encryption keys in regulated environments?
