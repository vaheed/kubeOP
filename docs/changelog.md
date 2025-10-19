# Changelog

All notable changes to this project are documented here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Removed
- Pruned unmaintained `samples/` templates to keep the repository lean.

### Added
- Repository sanity workflow validating module tidiness, duplicates, orphans, and Markdown links.

### Changed
- Moved the cleanup report under `docs/reports/` and expanded repository hygiene logging to surface pass/fail summaries.
- Formatted the version metadata source to satisfy Go formatting checks without manual tweaks.

## [0.15.0] - 2025-10-19

### Removed
- Deleted the remote event bridge component, related binaries, and database tables.
- Dropped event-bridge environment variables, deployment manifests, and ingest APIs.

### Changed
- Simplified documentation and configuration guides to reflect an API-only control plane.
- Updated CI, Dockerfiles, and tooling to build only the kubeOP API image.

## [0.14.16] - 2025-10-22

### Fixed
  elapse before retrying, preventing additional 401 loops while freshly
  issued credentials propagate through the control plane.
- Normalised the version metadata source file formatting so Go formatting
  checks pass without manual intervention during CI runs.

## [0.14.15] - 2025-10-21

### Fixed
  window expires, ensuring freshly issued credentials propagate before the next
  ingest attempt and stopping the alternating handshake/401 loop observed on
  kubeop-alborz clusters.

## [0.14.14] - 2025-10-20

### Fixed
  streaming events to another, eliminating the persistent 401 ingest loop observed on
  kubeop-alborz.
  claims (issuer, subject, audience, and expiry) so operators can diagnose auth mismatches
  without enabling verbose logging.

## [0.14.13] - 2025-10-19

### Fixed

## [0.14.12] - 2025-10-18

### Fixed
  refreshed credentials before the next retry, preventing the runaway 401 loops seen on eventually consistent clusters.

## [0.14.11] - 2025-10-18

### Fixed

## [0.14.10] - 2025-10-18

### Fixed

### Added
- End-to-end VitePress pages covering quickstart, configuration tables, operations runbook, and domain-specific guides.
- Zero-to-production operator guide plus consolidated API and kubectl references covering every endpoint and validation command.
- Automatic app domain lifecycle: kubeOP now issues `<app-full>.<project>.<cluster>.<PAAS_DOMAIN>` hostnames, provisions Let’s Encrypt TLS via cert-manager, persists domain metadata (including certificate status), and talks to pluggable DNS providers (`DNS_PROVIDER` + credentials for HTTP, Cloudflare, or PowerDNS) to upsert `A`/`AAAA` records on deploy and remove them on delete. `<app-full>` combines the slugified app name with a deterministic short hash of the app ID (for example, `web-02-f7f88c5b4-4ldbq`).

### Changed
- Relocated the security policy into `docs/security.md` and linked it from the README so Markdown layout requirements stay satisfied and the published docs mirror the repository structure.
- Default app hostnames now include the full deterministic app slug (`<name>-<short-hash>.<project>.<cluster>.<PAAS_DOMAIN>`) so generated domains always match the Kubernetes resource name (for example, `web-02-f7f88c5b4-4ldbq.alice-pg10.kubeop-alborz`).
- Rewrote API reference pages to mirror `internal/api` handlers, including request/response tables and curl examples that match current behaviour.
- Refreshed quickstart to document Docker Compose workflow, optional auth bypass, and app deployment verification.

### Fixed
- Pod Security defaults now expose `POD_SECURITY_WARN_LEVEL` and `POD_SECURITY_AUDIT_LEVEL` so operators can suppress warnings while keeping enforcement in sync with audit requirements.

## [0.14.8] - 2025-10-18

### Added
  tokens omit the `cluster_id` claim and that the sink adopts refreshed
  credentials after an unauthorized response.

### Fixed
  batches after 401s, falling back to token refresh only when bootstrap
  registration fails so persistent unauthorized loops clear automatically.
  eliminating the persistent 401 loop when queued batches retried before
  credentials rotation finished.
  events, and surfacing a `{"reason":"delivery"}` response until kubeOP accepts
  batches again.
  preventing persistent queue growth and keeping kubectl-driven changes in sync
  with kubeOP.
  observed token lifetime, tolerating clock skew between the control plane and
  remote clusters so event delivery no longer falls into 401 retry loops when
  clocks drift.
  existed, allowing existing deployments to reconnect without rotating
  secrets while still rejecting mismatched identifiers.
  records when tokens omit the claim so legacy deployments continue
  delivering events without manual credential rotation.
  clusters to stream events to `http://` control planes when the override is
  enabled.
- Fixed formatting of the version metadata file so Go formatting checks pass
  consistently in CI.
- Clarified kubeconfig lifecycle docs to reflect encryption, rotation, and secret deletion paths implemented in `internal/service/kubeconfigs.go`.
- Namespace bootstrap now applies ResourceQuota counts using Kubernetes `count/<resource>` identifiers and automatically drops
  incompatible quota scopes so clusters accept the managed `tenant-quota` without validation errors.
- Removed the default GPU extended resource limit so namespaces no longer require GPU capacity unless operators opt in via `KUBEOP_DEFAULT_LR_EXT_*`.

## [0.12.5] - 2025-12-06

### Changed

## [0.11.5] - 2025-12-05

### Changed

## [0.11.4] - 2025-12-04

### Fixed

## [0.11.3] - 2025-12-03

### Fixed

## [0.11.2] - 2025-12-02

### Fixed

## [0.11.0] - 2025-12-01

### Added

### Changed

## [0.10.8] - 2025-11-30

### Fixed

## [0.10.6] - 2025-11-29

### Fixed
  scheduler and ensuring fallback paths never run the deployer synchronously. Registration now responds immediately and
  surfaces rollout errors exclusively through logs.

## [0.10.5] - 2025-11-28

### Changed

## [0.10.4] - 2025-11-27

### Fixed

## [0.10.3] - 2025-11-26

### Added
  validation logic.

### Fixed
  Compose settings (`image: ghcr.io/vaheed/kubeop-api:latest`, `pull_policy: always`) that prevent the CLUSTER_ID error loop.

## [0.10.2] - 2025-11-25

### Fixed

## [0.10.1] - 2025-11-24

### Fixed
- Restored the public documentation site on GitHub Pages by setting the VitePress
  base path and introducing an automated deployment workflow that builds and
  publishes the docs from `main`.

## [0.10.0] - 2025-11-23

### Added

### Changed

### Fixed

## [0.9.2] - 2025-11-22

### Added

## [0.9.1] - 2025-11-21

### Fixed

## [0.9.0] - 2025-11-20

### Changed
- Removed the legacy `DEFAULT_LR_*` project LimitRange fallback variables and now seed explicit `PROJECT_LR_*` defaults, keeping namespace limit policy configuration singular.

### Removed
- Deprecated documentation and examples for `DEFAULT_LR_*`; operators should configure per-project limits exclusively via `PROJECT_LR_*`.

## [0.8.13] - 2025-10-24

### Added
- `/v1/apps/validate` endpoint returning load balancer quota checks, manifest summaries, and computed resource defaults so operators can confirm specs before deploying.
- Quickstart, API reference, and tutorial coverage showing how to run the validation workflow from curl.

### Changed
- README quickstart now includes a dry-run step ahead of kubeconfig rotation to highlight the validation workflow.

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

### Changed

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

### Fixed

## [0.8.1] - 2025-11-11

### Added

### Changed
