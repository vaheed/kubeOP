# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.0.1] - 2025-01-01
### Added
-
### Changed
-
### Removed
-
### Fixed
-

## [0.0.2] - 2025-10-30
### Added
- Structured E2E step telemetry persisted to `artifacts/e2e/<suite>-<test>.json`.

### Changed
- Hardened E2E tests with explicit tool detection, command error handling, and richer recovery assertions.
- Smoke and cluster endpoint suites share reusable helpers for artifact capture and result reporting.

### Fixed
- Documented the new E2E artifacts workflow in README and installation guide.
