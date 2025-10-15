# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- _Nothing yet_

### Changed
- _Nothing yet_

### Fixed
- _Nothing yet_

## [0.10.0] - 2025-11-23

### Added
- Asynchronous watcher deployment with background retries. Cluster registration
  now records watcher status transitions (`Pending`, `Deploying`, `Failed`,
  `Ready`) in the database and marks clusters unhealthy when the watcher misses a
  three minute readiness window.

### Changed
- Watcher configuration now centres on a single `WATCHER_URL` that accepts
  `http://` or `https://` endpoints (including custom ports) and derives the
  `/v1/events` and `/v1/health` targets automatically. The watcher maintains a
  persistent HTTP(S) connection for event delivery and polls `/v1/health` for
  lightweight liveness.
- The ingest sink reuses persistent HTTP connections instead of enforcing HTTPS
  only, matching the new watcher configuration semantics.

## [0.9.2] - 2025-11-22

### Added
- Startup now logs watcher auto-deploy status and reason, and cluster
  registrations record whether the watcher rollout ran or was skipped, making
  troubleshooting configuration mismatches easier.
- Configuration exposes the watcher auto-deploy source/explanation so tests and
  logs surface whether `publicURL`, config files, or environment overrides
  controlled the behaviour.

## [0.9.1] - 2025-11-21

### Fixed
- Watcher auto-deployment now enables automatically when `publicURL` is provided
  via configuration files, matching the environment variable behaviour so
  cluster registrations deploy the watcher bridge without additional flags.

## [0.9.0] - 2025-11-20

### Changed
- Removed the legacy `DEFAULT_LR_*` project LimitRange fallback variables and now seed explicit `PROJECT_LR_*` defaults, keeping namespace limit policy configuration singular.

### Removed
- Deprecated documentation and examples for `DEFAULT_LR_*`; operators should configure per-project limits exclusively via `PROJECT_LR_*`.

## [0.8.12] - 2025-11-19

### Added
- Helper for building namespace limit policy objects with unit tests so quota and limit defaults stay covered during future
  refactors.

### Changed
- Namespace limit policy reconciliation now flows through a reusable helper that reapplies the managed `tenant-quota` and
  `tenant-limits` objects on namespace provisioning, quota updates, and suspend/resume operations, ensuring drift is corrected
  automatically and emitting debug logs for operators.

## [0.8.11] - 2025-11-18

### Added
- Automatic NamespaceLimitPolicy bootstrap that applies annotated `tenant-quota` ResourceQuota and `tenant-limits` LimitRange objects with environment-driven defaults for every managed namespace.

### Changed
- Tenant kubeconfig roles now use a curated allow list of namespaced workloads and configuration resources instead of blanket
  wildcards, keeping access limited to the owning namespace.
- Documented the NamespaceLimitPolicy environment variables across README, `.env.example`, and `docs/ENVIRONMENT.md`, and updated the PR checklist to cover the new docs.

### Fixed
- Restored tenant kubeconfig permissions to allow scaling workloads via the `deployments/scale` and `statefulsets/scale` subresources, matching documented workflows.
- Corrected gofmt drift in the version metadata package to keep CI formatting checks green.

## [0.8.10] - 2025-11-17

### Added
- Introduced `GET /v1/projects/{id}/quota` to expose project ResourceQuota defaults, overrides, current usage, and load balancer caps for debugging quota errors.

### Changed
- Documented the quota snapshot endpoint across the README, API reference, quickstart, and quota guides to highlight how to inspect load balancer limits before patching overrides.

## [0.8.6] - 2025-11-16

### Changed
- Expanded `.env.example` with grouped documentation, Compose defaults, and
  explicit sample values so operators can configure kubeOP without cross
  referencing multiple files.
- Updated `docker-compose.yml` to source runtime settings exclusively from
  `.env`, eliminating duplicated credentials and connection strings.
- Refreshed the README with guidance on sharing `.env` between the API and the
  bundled PostgreSQL container.

## [0.8.5] - 2025-11-15

### Added
- Documented kubeOP and watcher network connectivity requirements so operators
  know which ports and URLs must be reachable.

### Changed
- Docker watcher image now declares port `8081` for health and metrics access,
  and CI builds push watcher-tagged images alongside the API image.
- Architecture diagrams now show the watcher’s HTTPS bridge back to kubeOP,
  clarifying the dependency on `PUBLIC_URL`.

## [0.8.4] - 2025-11-14

### Added
- Exposed `ClusterHealthScheduler.TickWithSummary` so operators and monitors
  can capture per-tick health statistics programmatically without parsing logs.

### Changed
- Final cluster health tick logs now include duration, failure counts, and start
  timestamps to support richer observability pipelines.

## [0.8.3] - 2025-11-13

### Added
- Cluster health scheduler now surfaces cluster identifiers and dependency
  warnings in logs to make health ticks actionable for operators.

### Changed
- Replaced string-based error logging in the scheduler with structured
  `zap.Error` fields for clearer diagnostics.

### Fixed
- Prevented scheduler ticks from continuing with missing dependencies by
  short-circuiting and logging actionable warnings instead of panicking.

## [0.8.2] - 2025-11-12

### Added
- Regression tests covering DNS log field helpers and Cloudflare provider error propagation to guard against regressions.

### Changed
- Routed DNS automation logs through the primary service logger with reusable helpers that attach project, app, cluster, service, and host context and expanded error annotations across synchronous and asynchronous wait paths.
- Surfaced Cloudflare API response payloads when ensuring or deleting records, with configurable HTTP clients for deterministic testing and richer operator feedback.
### Changed
- Watcher auto-deploy now remains disabled until `PUBLIC_URL` is configured,
  preventing development environments without HTTPS ingress from failing
  cluster registration.
- `.env.example` documents watcher-related configuration with sensible
  defaults, including the ingest URL and readiness tuning knobs.

### Fixed
- Configuration loading no longer seeds a placeholder `PUBLIC_URL`, avoiding
  unintended watcher rollouts that pointed at unreachable endpoints.

## [0.8.1] - 2025-11-11

### Added
- Automatic watcher bridge deployment from the API on cluster registration,
  minting per-cluster JWTs, annotating their SHA-256 fingerprints, and waiting
  for readiness before returning success.
- Dynamic derivation of the ingest endpoint from `PUBLIC_URL` so the bridge
  comes online without manual watcher configuration.
- Informer manager restart loop with exponential backoff so the watcher heals
  from transient startup failures and keeps health checks accurate.

### Changed
- Watcher auto deployment now defaults to enabled when kubeOP knows its public
  URL, and namespace creation/readiness waits default to on for zero-touch
  installs.
- Documentation for environment variables, README usage, and the watcher guide
  now highlights the automatic token handling, fingerprinting, and resilience
  improvements.

## [0.8.0] - 2025-11-10

### Added
- kubeOP Watcher Bridge (`cmd/kubeop-watcher`) for streaming labelled
  Kubernetes resource events to `/v1/events/ingest` with BoltDB-backed
  checkpoints, gzip batching, retries, Prometheus metrics, and
  heartbeat support.
- Watcher deployment documentation (`docs/WATCHER.md`) and README
  guidance, plus CI artifacts for both API and watcher binaries.

### Changed
- Dockerfile now builds both API and watcher images (multi-stage with a
  dedicated watcher target) and the Makefile/CI workflow builds both
  binaries by default.

## [0.7.0] - 2025-11-09

### Removed
- Dropped the deprecated `SA_TOKEN_TTL_SECONDS` setting now that kubeconfigs
  always use non-expiring ServiceAccount token Secrets.

### Changed
- Clarified tenancy documentation that kubeconfig tokens remain valid until
  rotation or revocation and refreshed the environment reference accordingly.

## [0.6.3] - 2025-11-08

### Fixed
- Wait for Service load balancer addresses before creating Cloudflare DNS records, logging progress and retrying asynchronously so app subdomains are published once an IP becomes available.

## [0.6.2] - 2025-11-07

### Added
- Guarded migration quality with `TestMigrationVersionsAreSequential`,
  ensuring every migration has a matching down file and contiguous,
  unique numbering.

### Changed
- API startup now emits explicit bootstrap logs and fails fast when
  logging configuration cannot initialise, avoiding partially configured
  runs.
- Roadmap and contributor guidance refreshed to match the current
  architecture, CI workflow, and documentation expectations.

### Fixed
- Renamed duplicate migration prefixes so golang-migrate applies schema
  changes deterministically across fresh and existing databases.

## [0.6.1] - 2025-11-06

### Fixed
- Capped `GET /v1/projects/{id}/logs` tail streaming to 5,000 lines to prevent excessive memory allocations and documented the API limit across OpenAPI, README, and reference guides.

## [0.6.0] - 2025-11-05

### Added
- Project event ingestion backed by PostgreSQL with JSONL fan-out to `${LOGS_ROOT}/projects/<project_id>/events.jsonl`, including cursor pagination and filterable API responses at `GET /v1/projects/{id}/events`.
- Custom event ingestion via `POST /v1/projects/{id}/events` and automatic emission for app lifecycle, config/secret attachments, quota updates, and kubeconfig renewals with actor attribution.

### Changed
- Admin middleware now surfaces the JWT subject or user identifiers on the request context so emitted project events carry `actor_user_id` details and redacted metadata in responses.

## [0.4.0] - 2025-11-02

### Added
- Per-user and per-project kubeconfig lifecycle APIs (`POST /v1/kubeconfigs`, `POST /v1/kubeconfigs/rotate`, `DELETE /v1/kubeconfigs/{id}`) with non-expiring ServiceAccount tokens backed by controller-managed Secrets and retry logic.
- Database persistence for kubeconfig bindings, including secret/service-account metadata for revocation tracking, plus unit tests covering the token minting wait-loop.

### Changed
- User bootstrap and project provisioning now mint kubeconfigs from `kubernetes.io/service-account-token` Secrets instead of TokenRequests, ensuring tokens remain valid until revoked and surfacing the mapping via the new API.
- Rotation and revocation update stored kubeconfigs, prune the previous Secret, and optionally delete the ServiceAccount when no other bindings remain.

## [0.3.17] - 2025-11-01

### Added
- Prometheus `readyz_failures_total{reason=...}` counter and structured WARN logs for readiness failures, including tests and documentation updates covering alert thresholds.

### Changed
- `/readyz` logging now annotates events (`event=readyz_failure|readyz_ok`) and the OpenAPI/README guidance highlights the readiness failure metric for operators.

## [0.3.16] - 2025-10-31

### Fixed
- Revalidated log directory joins with `filepath.Rel` so CodeQL sees every write anchored to `${LOGS_ROOT}` and traversal attempts fail before touching disk.
- Extended log helper tests to cover traversal edge cases, ensuring the sanitiser rejects empty, embedded, or `..` segments when touching files for projects/apps.

## [0.3.15] - 2025-10-30

### Fixed
- Routed log file creation through `${LOGS_ROOT}`-anchored helpers so CodeQL recognises sanitised segments and test helpers mimic production usage without exposing absolute path writes.

## [0.3.14] - 2025-10-29

### Fixed
- Enforced absolute, normalised log file paths before touching disk to close remaining CodeQL path expression alerts on the
  file-backed logger helper.

## [0.3.13] - 2025-10-28

### Fixed
- Restricted project and app log identifiers to `[A-Za-z0-9._-]+` and sanitised path joins so CodeQL can verify all disk-backed logs stay within `${LOGS_ROOT}`.

## [0.3.12] - 2025-10-27

### Fixed
- Normalised `${LOGS_ROOT}` and guarded file joins so disk-backed project/app logs cannot escape the configured root, addressing CodeQL path traversal alerts.

## [0.3.11] - 2025-10-26

### Fixed
- Sanitized project and app log identifiers so directories stay under `${LOGS_ROOT}`, trimming whitespace and rejecting path separators to close traversal paths reported by CodeQL.

## [0.3.10] - 2025-10-25

### Added
- Disk-backed project and application logging under `${LOGS_ROOT}/projects/<project_id>/` with per-app `app.log`/`app.err.log` plus aggregated `project.log` and `events.jsonl` files prepared on startup.
- `LOGS_ROOT` environment variable with Docker Compose mounting `./logs:/var/log/kubeop`, documentation updates, and tests exercising the file manager.

### Changed
- Sensitive key/value pairs (`password|token|secret|apikey|authorization`) are redacted by the writer while preserving JSON output across stdout, control-plane files, and project/app logs.

## [0.3.9] - 2025-10-24

### Added
- Production-grade zap logging with stdout + rotating files (`/var/log/kubeop/app.log` and `audit.log`) including RFC3339Nano timestamps, service/version metadata, and request IDs.
- HTTP access middleware capturing latency, byte counters, tenant/user hints, and returning `X-Request-Id` for correlation.
- Security audit middleware for mutating endpoints with automatic redaction of secrets/tokens/passwords and SIGHUP-triggered log reopen.
- Docker Compose volume mount (`./logs:/var/log/kubeop`) and environment documentation for log retention tuning.

### Changed
- Documentation (README, Operations, Environment, API reference, architecture) now describes log inspection workflows, request ID usage, and audit controls.

## [0.3.8] - 2025-10-23

### Changed
- Default Pod Security Admission level is now `baseline`, and container security defaults adapt to the configured level so
  root-required images run out of the box while `restricted` remains available for hardened tenants.
- Quickstart and application documentation now call out the security level trade-offs with explicit curl examples for both modes.

## [0.3.7] - 2025-10-22

### Fixed
- Soft-delete migrations now execute on fresh databases by removing unsupported `ALTER TABLE IF NOT EXISTS` syntax.
- Migration failures that leave the database dirty now include guidance to run `migrate force <version>` or reset the data volume before restarting.
- README, operations, and tests document and enforce the corrected migration behaviour for PostgreSQL 16+ setups.

## [0.3.6] - 2025-10-21

### Fixed
- Enforced default HTTP/S ports for Helm chart downloads, hardening the request pipeline against CodeQL-reported request forgery paths and adding contextual logging plus wrapped errors for observability.

## [0.3.5] - 2025-10-20

### Fixed
- Locked Helm chart downloads to the validated address list at dial time to prevent DNS rebinding or request forgery during chart rendering.

## [0.3.4] - 2025-10-19

### Fixed
- Hardened Helm chart downloads with a dedicated HTTP client that enforces host header integrity, blocks cross-host redirects, and adds redirect depth limits to stop request forgery attacks.

## [0.3.3] - 2025-10-18

### Fixed
- Validated Helm chart downloads resolve to globally routable addresses before issuing network requests, blocking SSRF via DNS rebinding or private resolution.

## [0.3.2] - 2025-10-17

### Fixed
- Rejected Helm chart downloads targeting localhost, loopback, or private networks and blocked non-http(s) schemes to prevent SSRF.
- Documented Helm chart URL guardrails across quickstart and apps guides and updated the PR checklist for external URL validation.

## [0.3.1] - 2025-10-16

### Added
- Structured readiness logging (`readyz status=...`) to aid dashboards and CI triage.
- Documentation plan refresh with roadmap alignment, open questions, and README/API/Operations updates.
- Project audit summary capturing production readiness recommendations.

### Changed
- README quickstart now clarifies readiness 503 behaviour and logging expectations for operators.
- CONTRIBUTING consolidates development workflow guidance previously duplicated in `docs/DEVELOPMENT.md` (removed).

### Fixed
- `/readyz` now returns HTTP 503 with `{"status":"not_ready","error":"service unavailable"}` when the service layer is missing instead of panicking.
- Kubeconfig YAML parsing logic deduplicated via `extractYAMLScalar` with regression tests to avoid drift between server and CA extraction.

## [0.3.0] - 2025-10-15

### Added
- Cluster health scheduler helper with bounded tick timeouts, structured logging, and unit coverage for scheduling edge cases.
- Manifest builders for tenant network policies and namespace RBAC, removing duplication and keeping specs consistent across provisioning paths.
- Documentation plan summarising audiences, gaps, and deliverables alongside an expanded roadmap and contributing guide updates.

### Changed
- Scheduler execution now runs through the shared helper and emits per-tick summaries for operators.
- README, architecture, API, environment, operations, security, and roadmap docs refreshed for production readiness with explicit next steps and open questions.

### Fixed
- Cluster health probes now inherit per-tick deadlines so a slow or unavailable cluster cannot stall the scheduler loop.
- Tenant namespace network policy and RBAC manifests are defined in one place, eliminating drift between project creation and user bootstrap flows.

## [0.2.1] - 2025-10-14

### Changed
- Stabilised kubeconfig user labels across bootstrap and renewal by reusing `service.ResolveUserLabel`, improving context readability for operators.
- Hardened quota override persistence with JSON-based helpers and regression tests to accept quoted values safely.
- Cluster health scheduler now logs lifecycle events and stops via context cancellation for predictable shutdowns.

### Fixed
- Renewed user kubeconfigs now preserve the original human-readable label instead of reverting to `user-sa`.
- `CreateProject` no longer masks database failures when fetching existing user spaces; unexpected errors surface to callers.

## [0.2.0] - 2025-10-13

### Added
- ConfigMap and Secret attachment endpoints for apps, including selective key
  support with optional prefixes and detach helpers that clean env references.
- Unit tests covering attachment helpers and API routing plus documentation for
  new flows across README, API reference, and quickstarts.
- CI artifact upload of the compiled API binary for reference alongside lint,
  build, and test steps.

### Changed
- Bumped API specification and version metadata to v0.2.0 and expanded PR
  checklist expectations for new endpoints.

## [0.1.3] - 2025-10-12

### Changed
- Consolidated Kubernetes app status collection and deployment mutation helpers
  to remove duplicated controller-runtime calls and emit warn-level logging when
  reads fail.

### Added
- Tests covering `service.CollectAppStatus` to exercise pod, service, and
  ingress summarisation paths.
- Contributor pull request checklist guidance to make required updates explicit.

## [0.1.2] - 2024-??-??

### Added
- Initial public release of the control plane baseline.
