Roadmap

Overview

- Five phases to reach a solid multi-tenant PaaS, prioritized by impact. Each item implies code, tests (under `testcase/`), and docs updates (including `docs/openapi.yaml`).
- Immediate next steps highlight the most actionable backlog based on the current code layout (service layer, scheduler helper, manifest builders, CI). Treat them as the default sprint plan.

Immediate Next Steps (0-2 sprints)

1. **Readiness instrumentation & alerting**
   - Emit Prometheus counters for readiness failures (`readyz_failures_total`) and log structured events to feed Grafana/Alertmanager.
   - Document alert thresholds and readiness failure triage flow in `docs/OPERATIONS.md` and `docs/METRICS.md`.
   - Provide sample dashboard JSON or screenshots once metrics exist.
2. **Scheduler observability**
   - Expose `/metrics` counters for `cluster_health_ticks_total`, `cluster_health_errors_total`, and duration histograms via the new scheduler helper.
   - Add structured log sampling or log level controls for noisy clusters; document expectations in `docs/METRICS.md` and `docs/OPERATIONS.md`.
3. **Manifest drift detection**
   - Use the shared manifest builders inside reconciliation logic that verifies tenant namespaces match desired policies.
   - Add tests stubbing controller-runtime fake clients to detect missing NetworkPolicies/RoleBindings.
4. **CI hardening**
   - Extend linting with `staticcheck` or `golangci-lint` presets for API packages and ensure coverage reports upload as artifacts.
   - Add a Compose-based smoke job that runs migrations + `/readyz` to guard against database regressions.
5. **Docs & runbooks**
   - Finish ENVIRONMENT/SECURITY updates (key rotation, SAToken TTL, DNS provider credentials) and link documentation plan updates in PR template.
   - Publish Grafana alert examples and cross-reference the documentation plan in README/CONTRIBUTING.


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
   - Allow scale subresources and common ops in-namespace: verbs `get,list,watch,create,update,patch,delete` on
     `deployments`, `replicasets`, `statefulsets`, `daemonsets`; subresources `deployments/scale`,
     `replicasets/scale`, `statefulsets/scale`; plus `pods/log`, `pods/exec`, and `pods/portforward` (rate-limited).
   - Acceptance: kubectl users in their namespace can scale and update workloads; no access outside their namespace.
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
2. Should `/readyz` expose a maintenance/read-only mode when Postgres is undergoing upgrades, and how is that communicated to clients?
3. How many clusters are expected in production, and do we need sharding or work queueing for the scheduler to keep tick durations bounded under load?
4. Which secrets management approach (external vault vs. Kubernetes secrets) is acceptable for kubeconfig encryption keys in regulated environments?
5. Should Docsify publishing move to an automated GitHub Pages workflow or remain manual?

Upcoming Phases — Future Releases

Phase — Unified Helm/OCI Engine

- Inputs: repo `{repo, chart, version}` or OCI `{chart: oci://..., version}`; reject `latest`.
- Use Helm SDK (no shell). Repo: in-memory repo add/index refresh; OCI: registry login from kubeOP SecretRefs.
- Values merge: defaults < chart values < user `valuesFiles` (ordered) < inline `values`.
- Namespacing: `createNamespace` option; ensure namespace exists.
- CRDs: install/upgrade with include-CRDs behavior.
- Release naming: `release = <projectSlug>-<appSlug>`; persist metadata (name, ns, chart ref, version, source, digest if OCI).
- Security/resilience: timeouts, retries, caching by repo+version or OCI digest; provenance verify (`.prov` or signature) when provided; block plain HTTP unless `ALLOW_INSECURE_REPOS=true`.
- Result: `{release, chart, version, digest?, namespace}`.
- Acceptance: deploy same app via repo or OCI by changing input; works offline after first pull (cache).

Phase — Lifecycle: Upgrade, Rollback, Status, Diff

- API: `POST /apps/:id/deploy`, `POST /apps/:id/rollback?rev=N`, `GET /apps/:id/status`, `GET /apps/:id/diff?toVersion=…`, `DELETE /apps/:id`.
- Upgrade: detect drift; only apply when chart/values changed; `wait`, `timeout`, `atomic=true` supported.
- Rollback: rollback to revision with reason; emit project events (DEPLOY, UPGRADE, ROLLBACK, UNINSTALL with severity).
- Status: release status, last revision, notes, workloads (deploy/rs/pods), conditions, last failure message.
- Diff: render current vs target; summarize changed kinds/names; size-limited output.
- Hooks/CRDs: expose notes; warn on incompatible CRDs and follow safe upgrade path.
- Observability: stepwise progress (fetch → render → apply → wait) to logs; store last rendered manifest (compressed) with retention/purge.
- Acceptance: upgrade modifies only changed objects; rollback restores previous revision; status shows accurate desired/ready counts.

Phase — Enterprise Hardening (Auth, Policy, Provenance, Tests)

- Auth & tenancy: per-project repo/OCI creds via SecretRefs; multi-cluster installs using project `cluster_id` kubeconfig; cache isolated per cluster.
- Security & policy: optional Gatekeeper/Kyverno dry-run pre-apply; block on deny with clear message; template preview mode `renderOnly=true` (no apply).
- Provenance: repo `.prov` verify with keyring; OCI signature/digest verify (cosign if available); record digest in release metadata.
- Reliability: exponential backoff on transient network/429; circuit breaker per repo/registry; clean retries without double-applying hooks; respect Helm atomic.
- Surface admission failures (PodSecurity/Quota) as project events with full K8s reason.
- Docs & tests: update `docs/APPS.md` with repo/OCI examples, values merge rules, rollback, diff, provenance flags; unit tests (validation, merge, resolver, error mapping); integration (kind + local registry: repo/OCI install, upgrade/rollback, signature verify, policy-deny path).
- Acceptance: credentials work per project; provenance optional; policy denies block apply and are reported; tests green.

Phase — LogStreamer (K8s Follow & Aggregation)

- Discovery: use client-go to list pods by labels `kubeop.project.id`, `kubeop.app.id`.
- Follow: `Pods(ns).GetLogs(...Follow:true, Timestamps:true)` for each container; dedupe streams by key `(cluster/ns/pod/container)`; reattach on restarts with exponential backoff + jitter.
- Envelope: wrap every line as JSON `{ts,cluster_id,namespace,project_id,project_name,app_id,app_name,pod,container,stream,line}`; redact when `LOG_REDACT_SECRETS=true`.
- Storage: write to `apps/<app_id>/app.log`, `project.log`, and `app.err.log` for stderr; respect RBAC; persist offsets in table `app_log_offsets`.
- Errors: on failure (evicted pod, permission), emit event `kind=LOG_STREAM_WARN`.
- Config: `LOG_AGGREGATION_ENABLED=true`, `LOG_FOLLOW_MAX_CLIENTS=50`, `LOG_REDACT_SECRETS=true`.
- Acceptance: `/logs` endpoint returns correct logs per filters; rotation and per-project isolation verified.

Phase — Events, APIs & K8s Bridge

- DB schema: `project_events(id, project_id, app_id, actor_user_id, kind, severity, message, meta jsonb, at timestamptz default now())` with indexes `(project_id, at desc)`, `(actor_user_id, at desc)`, and GIN on `meta`.
- Emit events: app lifecycle (create/update/delete/rollout), config/secret changes, policy rejections (quota, PodSecurity), mutating user actions (include `actor_user_id`), K8s core/v1 Events → normalized `K8S_EVENT`.
- Storage: append to DB and `/projects/<project_id>/events.jsonl`.
- APIs:
  - `GET /v1/projects/:id/logs` → `appId?`, `tail?`, `since?`, `follow?` (SSE).
  - `GET /v1/projects/:id/events` → filters (`kind,severity,actor,since`).
  - `GET /v1/projects/:id/apps/:appId/logs|status`.
  - `POST /v1/projects/:id/events` → append custom.
- Features: cursor pagination; grep/jq-style filters; per-project auth; throttle stream bandwidth; redact secrets in responses.
- Compose: add env `EVENTS_DB_ENABLED=true`, `K8S_EVENTS_BRIDGE=true`; mount `./logs:/var/log/kubeop`.
- Acceptance: events appear in DB and `events.jsonl`; `/logs` and `/events` return filtered data; rotation and auth isolation verified.

Namespace Drift & Change Audit (part of K8s Bridge)

- Watch user namespaces (informer caches) for `Deployments/ReplicaSets/StatefulSets/DaemonSets/Services/Ingresses/ConfigMaps/Secrets`.
- On `ADDED|MODIFIED|DELETED`, compute a concise diff summary and emit `PROJECT_CHANGE` event with `kind=K8S_RESOURCE_CHANGE` and object refs.
- Tag API-driven objects with `app.kubernetes.io/managed-by=kubeop` to separate platform intents from direct kubectl edits.
- Never auto-revert user edits; record and surface drift with links to logs/status; optionally allow “reconcile” action from API/UI later.
- SSE support: `/events` streams change events in near real-time with per-project auth and backpressure.
- Acceptance: editing via kubeconfig (kubectl apply/patch/scale) results in visible events and up-to-date app status within seconds.

Phase — Per-User Kubeconfig Lifecycle (Non-Expiring until Revoked)

- For each `<user, project>`: ensure Namespace; create ServiceAccount and Role/RoleBinding.
- Create Secret `kubernetes.io/service-account-token` with annotation `kubernetes.io/service-account.name=<sa>`; wait for controller to populate `data.token` and `data["ca.crt"]`.
- Build kubeconfig:
  - `users[].user.token = <secret.token>`
  - `clusters[].cluster.certificate-authority-data = base64(<ca.crt>)`
  - `clusters[].cluster.server = <api-server URL>`
  - `contexts[].context.namespace = <project-ns>`
- Persist mapping `<cluster_id, namespace, user_id, sa, secret>`.
- Endpoints: `POST /v1/kubeconfigs` (idempotent create/return), `POST /v1/kubeconfigs/rotate` (new token Secret), `DELETE /v1/kubeconfigs/{id}` (revoke by deleting Secret and SA if exclusive).
- Include RBAC templates; retries until Secret populated; unit/integration tests.
- Acceptance: kubeconfigs mint reliably without TokenRequest; rotation and revoke flows verified; audits/events emitted for mutations.
