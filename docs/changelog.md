# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- Updated architecture documentation with high-level, request lifecycle, watcher pipeline, and deployment topology diagrams built from current code paths.
- End-to-end VitePress pages covering quickstart, configuration tables, operations runbook, and domain-specific guides.

### Changed
- Rewrote API reference pages to mirror `internal/api` handlers, including request/response tables and curl examples that match current behaviour.
- Refreshed quickstart to document Docker Compose workflow, optional auth bypass, and app deployment verification.
- Consolidated watcher guidance across configuration, guides, and operations, clarifying auto-deploy prerequisites and manual setup.

### Fixed
- Clarified kubeconfig lifecycle docs to reflect encryption, rotation, and secret deletion paths implemented in `internal/service/kubeconfigs.go`.

## [0.10.1] - 2025-11-24

### Fixed
- Restored the public documentation site on GitHub Pages by setting the VitePress
  base path and introducing an automated deployment workflow that builds and
  publishes the docs from `main`.

## [0.10.0] - 2025-11-23

### Added
- `/v1/watchers/handshake` endpoint with authenticated readiness verification so watchers confirm connectivity before streaming events.
- Persistent watcher event queue backed by BoltDB, ensuring batches are cached locally when the API is unreachable and flushed automatically when connectivity returns.

### Changed
- Replaced `PUBLIC_URL`/watcher ingest URLs with a single `KUBEOP_BASE_URL` variable and new `ALLOW_INSECURE_HTTP` override for development.
- Watcher readiness now requires the state DB to open and a successful handshake within 60 seconds, returning JSON diagnostics when stale.
- Watcher startup logs and documentation reference the new base URL and handshake behaviour, aligning README, `.env.example`, and operations guidance.

### Fixed
- Ensured duplicate watcher events can be re-enqueued after persistence by resetting sink deduplication entries when batches fail.

## [0.9.2] - 2025-11-22

### Added
- Startup now logs watcher auto-deploy status and reason, and cluster registrations record whether the watcher rollout ran or was skipped, making troubleshooting configuration mismatches easier.
- Configuration exposes the watcher auto-deploy source/explanation so tests and logs surface whether `publicURL`, config files, or environment overrides controlled the behaviour.

## [0.9.1] - 2025-11-21

### Fixed
- Watcher auto-deployment now enables automatically when `publicURL` is provided via configuration files, matching the environment variable behaviour so cluster registrations deploy the watcher bridge without additional flags.

## [0.9.0] - 2025-11-20

### Changed
- Removed the legacy `DEFAULT_LR_*` project LimitRange fallback variables and now seed explicit `PROJECT_LR_*` defaults, keeping namespace limit policy configuration singular.

### Removed
- Deprecated documentation and examples for `DEFAULT_LR_*`; operators should configure per-project limits exclusively via `PROJECT_LR_*`.

## [0.8.12] - 2025-11-19

### Added
- Helper for building namespace limit policy objects with unit tests so quota and limit defaults stay covered during future refactors.

### Changed
- Namespace limit policy reconciliation now flows through a reusable helper that reapplies the managed `tenant-quota` and `tenant-limits` objects on namespace provisioning, quota updates, and suspend/resume operations, ensuring drift is corrected automatically and emitting debug logs for operators.

## [0.8.11] - 2025-11-18

### Added
- Automatic NamespaceLimitPolicy bootstrap that applies annotated `tenant-quota` ResourceQuota and `tenant-limits` LimitRange objects with environment-driven defaults for every managed namespace.

### Changed
- Tenant kubeconfig roles now use a curated allow list of namespaced workloads and configuration resources instead of blanket wildcards, keeping access limited to the owning namespace.
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
- Expanded `.env.example` with grouped documentation, Compose defaults, and explicit sample values so operators can configure kubeOP without cross referencing multiple files.
- Updated `docker-compose.yml` to source runtime settings exclusively from `.env`, eliminating duplicated credentials and connection strings.
- Refreshed the README with guidance on sharing `.env` between the API and the bundled PostgreSQL container.

## [0.8.5] - 2025-11-15

### Added
- Documented kubeOP and watcher network connectivity requirements so operators know which ports and URLs must be reachable.

### Changed
- Docker watcher image now declares port `8081` for health and metrics access, and CI builds push watcher-tagged images alongside the API image.
- Architecture diagrams now show the watcher’s HTTPS bridge back to kubeOP, clarifying the dependency on `PUBLIC_URL`.

## [0.8.4] - 2025-11-14

### Added
- Exposed `ClusterHealthScheduler.TickWithSummary` so operators and monitors can capture per-tick health statistics programmatically without parsing logs.

### Changed
- Final cluster health tick logs now include duration, failure counts, and start timestamps to support richer observability pipelines.

## [0.8.3] - 2025-11-13

### Added
- Cluster health scheduler now surfaces cluster identifiers and dependency warnings in logs to make health ticks actionable for operators.

### Changed
- Replaced string-based error logging in the scheduler with structured `zap.Error` fields for clearer diagnostics.

### Fixed
- Prevented scheduler ticks from continuing with missing dependencies by short-circuiting and logging actionable warnings instead of panicking.

## [0.8.2] - 2025-11-12

### Added
- Regression tests covering DNS log field helpers and Cloudflare provider error propagation to guard against regressions.

### Changed
- Routed DNS automation logs through the primary service logger with reusable helpers that attach project, app, cluster, service, and host context and expanded error annotations across synchronous and asynchronous wait paths.
- Surfaced Cloudflare API response payloads when ensuring or deleting records, with configurable HTTP clients for deterministic testing and richer operator feedback.
- Watcher auto-deploy now remains disabled until `PUBLIC_URL` is configured, preventing development environments without HTTPS ingress from failing cluster registration.
- `.env.example` documents watcher-related configuration with sensible defaults, including the ingest URL and readiness tuning knobs.

### Fixed
- Configuration loading no longer seeds a placeholder `PUBLIC_URL`, avoiding unintended watcher rollouts that pointed at unreachable endpoints.

## [0.8.1] - 2025-11-11

### Added
- Automatic watcher bridge deployment from the API on cluster registration, minting per-cluster JWTs, annotating their SHA-256 fingerprints, and waiting for readiness before returning success.
- Dynamic derivation of the ingest endpoint from `PUBLIC_URL` so the bridge comes online without manual watcher configuration.
- Informer manager restart loop with exponential backoff so the watcher heals from transient startup failures and keeps health checks accurate.

### Changed
- Watcher auto deployment now defaults to enabled when kubeOP knows its public URL, and namespace creation/readiness waits default to on for zero-touch installs.
- Documentation for environment variables, README usage, and the watcher guide now highlights the automatic token handling, fingerprinting, and resilience features shipped with the watcher.
