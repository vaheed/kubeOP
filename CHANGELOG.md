# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2025-10-31
### Added
- Deterministic CI harness (`.github/workflows/ci.yml`) with lint/unit/integration/e2e/coverage/summary jobs and shard-aware tooling under `tools/`.
- Postgres-backed integration tests and Kind-driven e2e smoke suites covering tenant→project→app flows, RBAC, quotas, delivery mocks, and billing/metrics validation.
- Documentation describing the CI pipeline and artifacts (`README.md`, `docs/ops/ci.md`).

### Changed
- Refactored e2e assets into `hack/e2e/` with bootstrap, teardown, and artifact collection scripts used exclusively by CI.
- Updated deployment samples and operator manifests to include quotas, network policy, and metrics/billing mocks consumed by tests.

### Fixed
- Ensured coverage aggregation and reporting fail the pipeline below 80% and capture controller diagnostics for debugging.

## [0.0.1] - 2025-01-01
### Added
- Initial release scaffold.