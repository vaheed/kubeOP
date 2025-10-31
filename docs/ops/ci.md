---
outline: deep
---

# Continuous Integration

kubeOP ships with a single workflow [`ci.yml`](https://github.com/vaheed/kubeOP/blob/develop/.github/workflows/ci.yml) that runs on every push to `main` or `develop` **and** on every pull request targeting those branches. The workflow orchestrates linting, tests, artifact capture, and coverage enforcement without relying on bespoke local tooling.

## Job overview

| Job | Purpose | Key artefacts |
| --- | --- | --- |
| `lint` | `go fmt`, `go vet`, and `golangci-lint` across all modules. | Lint logs (`lint.log`). |
| `unit` | Hermetic unit tests sharded deterministically via `tools/sharder`, executed with `-race` and `-tags=short`. | `unit-<n>.cov`, `unit-<n>.xml`. |
| `integration` | Postgres-backed tests (`-tags=integration`) with schema migrations and fixtures. | `integration-<n>.cov`, `integration-<n>.xml`. |
| `e2e` | Kind-per-shard suites invoking `hack/e2e/run.sh`, covering tenant→project→app flows, quotas, network policy, delivery mocks, and billing metrics. | JUnit XML, Kind dumps, controller logs, `/tmp/test-artifacts/**`, `coverage.cov`. |
| `coverage-merge` | Concatenates shard `.cov` files with `tools/covermerge` and publishes `coverage.out`. | `coverage.out`. |
| `summary` | Generates markdown summary (coverage %, slowest tests, flaky retries, artefact links). | `summary.md`. |

## Tags and sharding

- **short** – default unit suite. All packages without specialised build tags are executed with `-tags=short` in CI.
- **integration** – enables Postgres-backed tests. Requires `KUBEOP_DB_URL`; CI wires a GitHub Actions Postgres service.
- **e2e** – enables Kind-backed suites. The matrix splits specs via `KUBEOP_E2E_SUITE` so each job gets an isolated Kind cluster.

`tools/sharder` exposes `-total` and `-index` flags to split packages deterministically. E2E shards use regex selectors (`TestTenantsProjectsAppsFlow|TestRBACAndQuotas`, etc.) to keep cluster bootstraps bounded.

## Artifacts

Every job publishes deterministic outputs under predictable names:

- `junit/*.xml` – per-job suites, merged later by `tools/junitmerge`.
- `coverage/*.cov` – shard-specific coverage segments.
- `kind/cluster-*.log`, `logs/<ns>/*.log`, `cr-*.yaml` – produced by `hack/e2e/collect.sh`.
- `/tmp/test-artifacts/**` – additional state captured by the e2e harness (e.g., `go-test.json`).

The `summary` job asserts total coverage ≥ 80% and fails otherwise. Flaky suites are retried once, tagged in the summary, and surfaced as warnings in the PR checklist.
