# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]
### Changed
- Documented auxiliary HTTP binaries (`admission`, `delivery`, `meter`, `healthcheck`) and mock services in the README and code-sourced docs.

### Removed
- Deleted the generated `docs/package-lock.json` to keep the documentation site dependency lockfiles ephemeral.

## [0.0.2] - 2025-01-15
### Added
- Rebuilt documentation as a VitePress site generated from the current Go sources (new `docs/` tree, `DOCS.md`, and updated README).
- Added ready-to-apply example manifests under `examples/tenant-project-app` covering Tenant, Project, App, DNSRecord, and Certificate resources.
- Documented manager API endpoints, controller flows, CRD schemas, configuration, operations, security, and troubleshooting with direct links to code.
- Updated pull request template to enforce tests, linting, and docs rebuilds.

### Changed
- Bumped project version to `v0.0.2`.

### Removed
- Legacy documentation (`/docs`, `CONTRIBUTING.md`, `SECURITY.md`, `AGENTS.md`) superseded by code-sourced content.

## [0.0.1] - 2025-01-01
### Added
- Initial public release.
