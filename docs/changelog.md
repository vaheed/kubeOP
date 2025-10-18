# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- Updated architecture documentation with high-level, request lifecycle, watcher pipeline, and deployment topology diagrams built from current code paths.
- End-to-end VitePress pages covering quickstart, configuration tables, operations runbook, and domain-specific guides.
- Zero-to-production operator guide plus consolidated API and kubectl references covering every endpoint and validation command.
- Automatic app domain lifecycle: kubeOP now issues `<app-full>.<project>.<cluster>.<PAAS_DOMAIN>` hostnames, provisions Let’s Encrypt TLS via cert-manager, persists domain metadata (including certificate status), and talks to pluggable DNS providers (`DNS_PROVIDER` + credentials for HTTP, Cloudflare, or PowerDNS) to upsert `A`/`AAAA` records on deploy and remove them on delete. `<app-full>` combines the slugified app name with a deterministic short hash of the app ID (for example, `web-02-f7f88c5b4-4ldbq`).
- Watcher bootstrap API with `/v1/watchers/register` + `/v1/watchers/refresh`, short-lived JWTs, and persisted refresh tokens. Watchers now rotate credentials automatically and mark readiness only after a successful handshake.
- Expanded roadmap with Phase 4F "Job & Schedule Management" covering APIs, watcher integration, scheduling, UI, billing, security, and documentation workstreams for batch workloads.

### Changed
- Relocated the security policy into `docs/security.md` and linked it from the README so Markdown layout requirements stay satisfied and the published docs mirror the repository structure.
- Default app hostnames now include the full deterministic app slug (`<name>-<short-hash>.<project>.<cluster>.<PAAS_DOMAIN>`) so generated domains always match the Kubernetes resource name (for example, `web-02-f7f88c5b4-4ldbq.alice-pg10.kubeop-alborz`).
- Rewrote API reference pages to mirror `internal/api` handlers, including request/response tables and curl examples that match current behaviour.
- Refreshed quickstart to document Docker Compose workflow, optional auth bypass, and app deployment verification.
- Consolidated watcher guidance across configuration, guides, and operations, clarifying auto-deploy prerequisites and manual setup.
- Watcher deployer now sets `imagePullPolicy: Always` and injects `KUBEOP_BASE_URL`/`ALLOW_INSECURE_HTTP` alongside legacy `KUBEOP_EVENTS_URL` for compatibility. Nodes always pull from GHCR and both new and old watcher images work.
- Align code style with `gofmt` for watcher deployer and related tests (no functional changes).
- Watcher bridge filters namespaces via `WATCH_NAMESPACE_PREFIXES` and now relies on existing kubeOP labels instead of stamping them automatically, keeping manual workloads decoupled unless operators opt in.
- Pod Security defaults now expose `POD_SECURITY_WARN_LEVEL` and `POD_SECURITY_AUDIT_LEVEL` so operators can suppress warnings while keeping enforcement in sync with audit requirements.
- Watcher `/readyz` now serves HTTP 200 with `status: degraded` and diagnostic fields when upstream handshakes or deliveries fail, keeping pods running offline while events buffer locally.
- Watcher ingest no longer auto-creates kubeOP projects or apps for workloads deployed via `kubectl`; events require existing kubeOP project labels and manual workloads remain unmanaged by design.

### Fixed
- Watcher readiness now treats ingest failures as delivery issues instead of
  handshake errors, keeping the watcher running offline, preserving queued
  events, and surfacing a `{"reason":"delivery"}` response until kubeOP accepts
  batches again.
- Watcher sink retries immediately after 401 responses once credentials refresh,
  preventing persistent queue growth and keeping kubectl-driven changes in sync
  with kubeOP.
- Watcher access-token refreshes now schedule well ahead of expiry using the
  observed token lifetime, tolerating clock skew between the control plane and
  remote clusters so event delivery no longer falls into 401 retry loops when
  clocks drift.
- `/v1/watchers/handshake` now falls back to the request payload
  `cluster_id` when validating watcher tokens minted before the claim
  existed, allowing existing deployments to reconnect without rotating
  secrets while still rejecting mismatched identifiers.
- Watcher event ingest now backfills missing `cluster_id` claims from
  persisted watcher metadata so legacy tokens minted without the claim
  continue delivering events after upgrades.
- Watcher event sink now honours `ALLOW_INSECURE_HTTP`, allowing development
  clusters to stream events to `http://` control planes when the override is
  enabled.
- Watcher authentication now resolves cluster-scoped tokens that omit the
  `watcher_id` claim, eliminating 401 responses from `/v1/events/ingest` and
  `/v1/watchers/handshake` when legacy bootstrap secrets are used.
- Fixed formatting of the version metadata file so Go formatting checks pass
  consistently in CI.
- Clarified kubeconfig lifecycle docs to reflect encryption, rotation, and secret deletion paths implemented in `internal/service/kubeconfigs.go`.
- Namespace bootstrap now applies ResourceQuota counts using Kubernetes `count/<resource>` identifiers and automatically drops
  incompatible quota scopes so clusters accept the managed `tenant-quota` without validation errors.
- Removed the default GPU extended resource limit so namespaces no longer require GPU capacity unless operators opt in via `KUBEOP_DEFAULT_LR_EXT_*`.
- Watcher deployments pin `LOGS_ROOT=/var/lib/kubeop-watcher/logs` and surface the override so non-root pods no longer fail when they cannot create `/var/log/kubeop`.

## [0.12.5] - 2025-12-06

### Changed
- Published container images now target dedicated packages (`ghcr.io/<owner>/kubeop-api` and `ghcr.io/<owner>/kubeop-watcher`) so `:latest`, branch, and SemVer tags remain unique per binary. Defaults, Compose references, and documentation now reference the new package names.

## [0.11.5] - 2025-12-05

### Changed
- Watcher deployments keep the hardened `restricted` defaults while allowing operators to override the numeric UID/GID/FSGroup via `WATCHER_RUN_AS_USER`, `WATCHER_RUN_AS_GROUP`, and `WATCHER_FS_GROUP`; the generated Deployment and container security contexts honour the overrides while still defaulting to `65532`.

## [0.11.4] - 2025-12-04

### Fixed
- Watcher deployments now pin UID/GID/FSGroup `65532` and the container image runs as the `nonroot` user, eliminating `CreateContainerConfigError` failures on clusters that require explicit numeric identities for `runAsNonRoot` pods.

## [0.11.3] - 2025-12-03

### Fixed
- Watcher deployment now applies PodSecurity `restricted` defaults (run as non-root, drop all capabilities, disable privilege escalation, runtimeDefault seccomp) so restarts no longer trigger admission warnings.

## [0.11.2] - 2025-12-02

### Fixed
- Watcher pods now probe configured kinds and skip those whose Kubernetes APIs are unavailable, preventing CrashLoopBackOff when optional CRDs such as cert-manager are missing and keeping readiness green for the remaining resources.

## [0.11.0] - 2025-12-01

### Added
- Implemented `POST /v1/events/ingest` to accept watcher batches when `K8S_EVENTS_BRIDGE=true`, normalising Kubernetes changes into project events with structured logging and per-batch acceptance metrics.

### Changed
- Updated watcher documentation (README, configuration, operations, guides, API reference) to reflect the live ingest endpoint, label compatibility, and bridging requirements.

## [0.10.8] - 2025-11-30

### Fixed
- Watcher sinks configured with a persistent queue now persist failed batches after the first attempt, preventing tight retry loops against the control plane while connectivity is degraded.

## [0.10.6] - 2025-11-29

### Fixed
- Prevented watcher rollout readiness failures from blocking cluster registration by lazily initialising the asynchronous
  scheduler and ensuring fallback paths never run the deployer synchronously. Registration now responds immediately and
  surfaces rollout errors exclusively through logs.

## [0.10.5] - 2025-11-28

### Changed
- Cluster registration now queues watcher auto-deploy in the background so the API returns immediately while readiness checks continue asynchronously. Structured logs (`queueing watcher deployment ensure`, `starting watcher ensure`, `watcher ensure complete`) capture progress and surface failures without holding the HTTP request open.

## [0.10.4] - 2025-11-27

### Fixed
- Forced Docker Compose to pull the published API image by default (`image: ghcr.io/vaheed/kubeop-api:latest`, `pull_policy: always`) while still building the `api` stage locally, and documented removing stale watcher tags so `docker compose up` stops launching the watcher binary that fails with the `CLUSTER_ID` error.

## [0.10.3] - 2025-11-26

### Added
- Extracted the watcher handshake HTTP helper into `internal/watcher/handshake` with unit coverage so future agents share the
  validation logic.

### Fixed
- Expanded watcher startup diagnostics to call out when the watcher image is launched in place of the API and documented the
  Compose settings (`image: ghcr.io/vaheed/kubeop-api:latest`, `pull_policy: always`) that prevent the CLUSTER_ID error loop.

## [0.10.2] - 2025-11-25

### Fixed
- Prevented Docker Compose from launching the watcher image (which exits with `config error: CLUSTER_ID is required`) by pinning the build to the API stage for local development.

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
