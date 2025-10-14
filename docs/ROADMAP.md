# Roadmap

The roadmap focuses on keeping the current Go API, PostgreSQL store, and
multi-cluster scheduler production ready while iterating in small,
verifiable releases. Each milestone assumes code updates under `cmd/`
and `internal/`, matching tests inside `testcase/`, documentation changes
(including OpenAPI when APIs move), and migration validation.

## Vision

- Deliver a tenant-safe control plane that provisions namespaces,
  credentials, and applications across multiple Kubernetes clusters.
- Fail fast on misconfiguration, surface actionable logs/metrics, and
  keep operators in control of upgrades.
- Prefer boring infrastructure: standard PostgreSQL, deterministic
  migrations, and stateless API replicas.

## Current architecture snapshot

- **API**: `cmd/api` exposes REST routes via `internal/api` with auth
  middleware, service wiring, and health probes.
- **Service layer**: `internal/service` orchestrates kubeconfig lifecycle,
  project/app provisioning, events fan-out, and background scheduling.
- **Store**: `internal/store` persists clusters, users, projects, apps,
  kubeconfigs, and events through pgx with embedded migrations.
- **Scheduler**: `service.NewClusterHealthScheduler` polls clusters on a
  configurable interval and reports readiness.
- **Logging & metrics**: `internal/logging` configures zap + lumberjack,
  while `internal/metrics` exposes Prometheus counters and gauges.

## Immediate priorities (Weeks 0–4)

1. **Migration hygiene**
   - Keep migration numbering contiguous, unique, and covered by
     `TestMigrationVersionsAreSequential`.
   - Document every schema change in `docs/CHANGELOG.md` and create
     downgrade plans for operators.
2. **Startup reliability**
   - Fail fast when logging, configuration, or migrations cannot be
     initialised (already enforced in `cmd/api/main.go`).
   - Add smoke tests that run migrations + `/readyz` in CI.
3. **Scheduler observability**
   - Emit counters/histograms for cluster health runs and expose them in
     Grafana dashboards (`docs/dashboards`).
   - Document remediation steps in `docs/OPERATIONS.md`.
4. **Documentation refresh**
   - Keep quickstarts aligned with the latest auth, kubeconfig, and log
     behaviour.
   - Expand runbooks for PostgreSQL maintenance and log rotation.

## Near-term milestones (1–3 months)

- **Tenant lifecycle polish**
  - Harden kubeconfig rotation and revocation flows; ensure namespace
    annotations/labels stay in sync.
  - Add pagination and filtering to project/app listings.
- **Application delivery UX**
  - Finalise Helm/OCI renderer consolidation with digest pinning and
    per-release metadata stored in PostgreSQL.
  - Extend app status aggregation to surface rollout blocks and recent
    events via the API.
- **Security & isolation**
  - Expand RBAC defaults for day-2 operations; tighten NetworkPolicy
    templates; add regression tests.
  - Wire optional OpenTelemetry tracing with sampling controls.

## Maintainability & scalability initiatives

- **Code organisation**: Extract reusable helpers inside
  `internal/service` (e.g., manifest builders, event emitters) to reduce
  duplication across HTTP handlers and scheduler jobs.
- **Performance**: Profile store queries; add indexes for frequent reads
  (`deleted_at IS NULL`, project/app lookups). Document results in
  `docs/OPERATIONS.md`.
- **Testing**: Introduce focused integration suites that run against a
  disposable Postgres in CI (use sqlmock for unit paths, Docker for
  migration coverage).
- **Configuration hygiene**: Validate environment variables on startup
  and surface warnings for deprecated or unused keys.
- **Scalability**: Prepare for multiple cluster managers by queuing
  scheduler work and bounding per-tick latency; plan sharding strategy in
  `docs/ARCHITECTURE.md`.

## Future opportunities (3–6 months)

- Self-service build pipelines (Buildpacks/Kaniko) with image provenance
  checks.
- Tenant-facing dashboards summarising quota usage, recent events, and
  rollout health.
- Multi-cluster placement policies with affinity/anti-affinity and
  pluggable schedulers.
- Enterprise hardening: SSO integration, per-tenant audit exports, and
  policy-driven deployment approvals.

## Operational checkpoints

- Keep `.github/workflows/ci.yml` aligned with required checks (install
  deps, vet, build, test, artifact upload).
- Update the PR template checklist whenever workflow or documentation
  expectations change.
- Revisit this roadmap quarterly; archive delivered work and add
  actionable next steps tied to architecture reality.

