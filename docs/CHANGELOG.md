# Changelog

All notable changes to this project are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added
- _Nothing yet_

### Fixed
- _Nothing yet_

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
