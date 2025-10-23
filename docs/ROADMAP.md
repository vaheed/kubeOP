# Roadmap

The roadmap reflects the current state of the kubeOP codebase and prioritises work that
completes partially implemented features, closes operational gaps, and prepares the
project for the next release. Each initiative maps directly to existing packages,
APIs, or data models in this repository.

## Prioritisation (RICE)

| Rank | Initiative | Track | Phase | Reach | Impact | Confidence | Effort | RICE | Issue |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | Operator parity for App CRD features | Delivery | Near-term | 320 | 3 | 0.7 | 2 | 336 | [#ROAD-1](https://github.com/vaheed/kubeOP/issues/ROAD-1) |
| 2 | Release detail & diff endpoints | API | Near-term | 220 | 2 | 0.8 | 1 | 352 | [#ROAD-2](https://github.com/vaheed/kubeOP/issues/ROAD-2) |
| 3 | Project event retention & indexing | Data | Near-term | 280 | 2 | 0.65 | 2 | 182 | [#ROAD-3](https://github.com/vaheed/kubeOP/issues/ROAD-3) |
| 4 | Backup & restore automation runbooks | Ops | Near-term | 180 | 2 | 0.7 | 1 | 252 | [#ROAD-4](https://github.com/vaheed/kubeOP/issues/ROAD-4) |
| 5 | Harden admin auth & rotate secrets | Security | Near-term | 160 | 3 | 0.6 | 2 | 144 | [#ROAD-5](https://github.com/vaheed/kubeOP/issues/ROAD-5) |
| 6 | Docs release governance automation | Docs | Mid-term | 140 | 2 | 0.6 | 1 | 168 | [#ROAD-6](https://github.com/vaheed/kubeOP/issues/ROAD-6) |

> **Rationale:** Reach approximates the number of active clusters and projects touched by
each change (derived from `internal/store` usage patterns and sample data); impact
reflects service availability or operator productivity improvements; confidence stems
from existing tests in `testcase/`; effort is measured in engineer-weeks (S=0.5,
M=1, L=2+).

## Phase: Near-term (0–6 weeks)

### Delivery Track — Operator parity for App CRD features
- **Problem**: The in-cluster operator at `kubeop-operator/controllers/app_controller.go`
  only reconciles Deployments, while the control plane supports config/secret
  attachments, services, and load-balancer metadata via `internal/service/apps.go`.
  Divergence leaves App CRD users without parity or status updates.
- **Outcome**: App CRDs reconcile the same resources and status conditions that the
  direct Deployment path manages, ensuring SBOM metadata, env attachments, and
  rollout controls stay in sync.
- **Scope (In)**: Extend desired-state builders to include Services, ConfigMaps,
  Secrets, and annotations; persist status parity through
  `internal/service/mirrorAppCRD`; add regression tests under
  `kubeop-operator/controllers/app_controller_test.go` and `testcase/service_manifests_test.go`.
- **Scope (Out)**: UI surfaces, new delivery sources, or Helm templating changes.
- **Acceptance Criteria**:
  - CRD-driven apps expose the same env/config references as recorded in
    `store.AppTemplate` and `store.App` rows.
  - Operator emits status updates that match `internal/service` expectations for
    `AppConditionReady`.
  - Added tests cover ConfigMap/Secret attachment and Service reconciliation.
  - Docs in `docs/API.md` and `docs/OPERATIONS.md` explain CRD parity and rollout
    guidance.
- **Dependencies**: Existing deployment helpers in `internal/service`; Kubernetes API
  machinery dependencies already vendored in `kubeop-operator/go.mod`.
- **Risks**: Increased controller-runtime complexity; potential drift between
  Deployment and CRD code paths.
- **Estimation**: L (two sprints).
- **Owner**: _TBD_
- **Issue**: [#ROAD-1](https://github.com/vaheed/kubeOP/issues/ROAD-1)

### API Track — Release detail & diff endpoints
- **Problem**: `internal/api/releases.go` only lists releases. Operators cannot fetch
  a single release or compare revisions despite rich data in
  `internal/store/releases.go`.
- **Outcome**: `/v1/projects/{id}/apps/{appId}/releases/{releaseId}` exposes full
  metadata (render digests, SBOM, load balancers), and a diff helper shows field
  changes between releases.
- **Scope (In)**: Add handler/service/store wiring; expose OpenAPI schema updates in
  `docs/openapi.yaml`; document usage in `docs/API.md` and `README.md` examples.
- **Scope (Out)**: UI visualisation, webhook notifications, or Git compare tooling.
- **Acceptance Criteria**:
  - Handler returns `404` for missing releases and `409` on project/app mismatch.
  - Diff helper highlights spec, image, and LB changes with tests in
    `testcase/service_releases_test.go` and new API tests.
  - Docs and snippets include `curl` examples.
- **Dependencies**: Existing `store.GetRelease` and hashing helpers.
- **Risks**: Payload size growth; need for pagination on diff history.
- **Estimation**: M.
- **Owner**: _TBD_
- **Issue**: [#ROAD-2](https://github.com/vaheed/kubeOP/issues/ROAD-2)

### Data Track — Project event retention & indexing
- **Problem**: `project_events` currently grows unbounded and lacks composite indexes,
  impacting `ListProjectEvents` queries in `internal/store/events.go` once ingest
  volume increases.
- **Outcome**: Time-based retention, indexed queries, and archival exports keep
  ingestion fast and storage bounded.
- **Scope (In)**: Add migrations in `internal/store/migrations`; implement retention
  workers or SQL TTL (e.g., partitioning); document knobs in `docs/ENVIRONMENT.md`;
  add metrics in `internal/metrics`.
- **Scope (Out)**: External data lake integration or UI.
- **Acceptance Criteria**:
  - Queries stay <200ms for 50k events in tests.
  - Retention configuration defaults documented and enforced in service layer.
  - Scheduler summarises purged counts with structured logs.
- **Dependencies**: `service.IngestProjectEvents`, background scheduler wiring in
  `cmd/api/main.go` (reuse existing context cancellation patterns).
- **Risks**: Data loss if retention misconfigured; migration downtime.
- **Estimation**: M.
- **Owner**: _TBD_
- **Issue**: [#ROAD-3](https://github.com/vaheed/kubeOP/issues/ROAD-3)

### Ops Track — Backup & restore automation runbooks
- **Problem**: Operational docs lack actionable procedures for database backups,
  log directory rotation, or verifying cluster health scheduler output.
- **Outcome**: Operators follow scripted procedures (Makefile/CLI) with logging in
  `internal/logging` and new docs sections.
- **Scope (In)**: Add Makefile targets for pg_dump/restore; document scheduling of
  cluster health checks; ship sample cronjobs in `docs/examples/`; expand
  `docs/OPERATIONS.md` and `docs/TROUBLESHOOTING.md`.
- **Scope (Out)**: Managed service integrations or vendor-specific tooling.
- **Acceptance Criteria**:
  - Scripts emit structured logs and exit codes.
  - Restore drill tested via `docker-compose` sample.
  - Docs provide verification commands and rollback steps.
- **Dependencies**: Existing `Makefile`, Docker Compose sample, `logging` helpers.
- **Risks**: Backup scripts leaking secrets; ensuring cross-platform compatibility.
- **Estimation**: M.
- **Owner**: _TBD_
- **Issue**: [#ROAD-4](https://github.com/vaheed/kubeOP/issues/ROAD-4)

### Security Track — Harden admin auth & rotate secrets
- **Problem**: Default admin JWT secret and `DisableAuth` switch in
  `internal/config/config.go` allow insecure deployments; no guidance for key
  rotation.
- **Outcome**: Secure-by-default admin auth with rotation tooling and explicit
  migration notes.
- **Scope (In)**: Remove or deprecate `DisableAuth`; enforce strong secret on boot;
  add rotation endpoints/tests in `internal/api/auth.go`; document runbook in
  `docs/SECURITY.md` and env refs.
- **Scope (Out)**: Full multi-tenant RBAC (tracked separately in Mid-term).
- **Acceptance Criteria**:
  - Server refuses to start without non-default secret.
  - Rotation flow logged and tested (`testcase/api_auth_test.go`).
  - Deprecation notice for `DisableAuth` includes migration steps for
    air-gapped dev installs.
- **Dependencies**: Config loader, logging, maintenance guard.
- **Risks**: Breaking local setups; handling secret distribution securely.
- **Estimation**: M.
- **Owner**: _TBD_
- **Issue**: [#ROAD-5](https://github.com/vaheed/kubeOP/issues/ROAD-5)
- **Deprecation**: `DisableAuth` environment flag — operators must migrate to the
  rotation endpoint; provide sample manifests for secret management.

### Docs Track — (No additional Near-term docs initiatives)
- Current sprint already delivers the IA refresh and release policy; further docs
  automation is planned in Mid-term.

## Phase: Mid-term (6–12 weeks)

### Delivery Track — Template versioning & validation
- **Problem**: `internal/store/templates.go` stores a single version per template,
  preventing safe updates.
- **Outcome**: Versioned templates with validation diffs and upgrade hooks.
- **Scope (In)**: Extend template models/migrations; update `internal/service/templates.go`;
  add CLI/docs guidance.
- **Scope (Out)**: UI for template review.
- **Acceptance Criteria**: Version history API, downgrade guardrails, tests in
  `testcase/service_templates_test.go`.
- **Dependencies**: Release diff work, migration tooling.
- **Risks**: Schema churn, upgrade complexity.
- **Estimation**: L.
- **Owner**: _TBD_
- **Issue**: [#ROAD-7](https://github.com/vaheed/kubeOP/issues/ROAD-7)

### API Track — Tenant-scoped API tokens & RBAC
- **Problem**: All admin operations rely on a single JWT secret; tenants cannot
  self-serve safely.
- **Outcome**: Introduce scoped tokens tied to users/projects leveraging existing
  kubeconfig issuance logic.
- **Scope (In)**: Extend `internal/api/auth.go`, `internal/service/kubeconfigs.go`, and
  `docs/API.md` to support token issuance and auditing.
- **Scope (Out)**: UI login portal (Later phase).
- **Acceptance Criteria**: Token CRUD endpoints, audit logging, tests under
  `testcase/api_auth_test.go`.
- **Dependencies**: Security track secret rotation.
- **Risks**: Token leakage, ensuring compatibility with maintenance mode.
- **Estimation**: L.
- **Owner**: _TBD_
- **Issue**: [#ROAD-8](https://github.com/vaheed/kubeOP/issues/ROAD-8)

### Data Track — Metrics & tracing instrumentation
- **Problem**: Observability limited to readiness counters and logs; scheduler and
  delivery pipelines lack metrics (`internal/metrics`, `service.NewClusterHealthScheduler`).
- **Outcome**: Prometheus counters/histograms and optional OpenTelemetry spans cover
  API latency, delivery validation, and scheduler ticks.
- **Scope (In)**: Extend `internal/metrics`, wrap key service methods with timers,
  document config toggles.
- **Scope (Out)**: External tracing backends (exporters only).
- **Acceptance Criteria**: Metrics exposed under `/metrics`; docs include Grafana
  dashboards; tests validate counters in `testcase/metrics_readyz_test.go`.
- **Dependencies**: None beyond existing instrumentation.
- **Risks**: Performance overhead; exporter configuration complexity.
- **Estimation**: M.
- **Owner**: _TBD_
- **Issue**: [#ROAD-9](https://github.com/vaheed/kubeOP/issues/ROAD-9)

### Ops Track — Operator canary upgrade automation
- **Problem**: Operator deployment automation in `internal/service/kube.go` lacks
  canary or rollback strategy.
- **Outcome**: Implement staged rollouts with health checks per cluster.
- **Scope (In)**: Add upgrade orchestration to service layer; sample manifests and
  docs updates.
- **Scope (Out)**: Multi-tenant UI for upgrades.
- **Acceptance Criteria**: Canary toggles, rollback CLI command, tests in
  `testcase/service_clusters_test.go`.
- **Dependencies**: Delivery parity work.
- **Risks**: Upgrade failures leaving clusters inconsistent.
- **Estimation**: M.
- **Owner**: _TBD_
- **Issue**: [#ROAD-10](https://github.com/vaheed/kubeOP/issues/ROAD-10)

### Security Track — External KMS integration for secrets
- **Problem**: `internal/crypto` uses derived keys from env vars; no KMS support for
  storing kubeconfig or credential secrets.
- **Outcome**: Pluggable KMS backend (e.g., HashiCorp Vault, AWS KMS) for
  encryption/decryption flows.
- **Scope (In)**: Abstract `crypto` package, add config toggles, update docs.
- **Scope (Out)**: UI key management.
- **Acceptance Criteria**: Integration tests with fake KMS, fallback to existing AES.
- **Dependencies**: Config loader enhancements.
- **Risks**: Increased complexity, vendor lock-in.
- **Estimation**: L.
- **Owner**: _TBD_
- **Issue**: [#ROAD-11](https://github.com/vaheed/kubeOP/issues/ROAD-11)

### UX Track — CLI parity for high-frequency admin flows
- **Problem**: Operations rely on raw `curl`; `docs/CLI.md` documents flags but no
  packaged tooling.
- **Outcome**: Minimal Go or Node CLI using existing API clients for onboarding.
- **Scope (In)**: Create CLI under `cmd/cli` (new module), provide installers, update
  docs.
- **Scope (Out)**: Web portal (Later phase).
- **Acceptance Criteria**: CLI covers cluster register, project bootstrap, app deploy;
  tests in `testcase` verifying command output.
- **Dependencies**: API enhancements for releases.
- **Risks**: Binary distribution overhead, version skew.
- **Estimation**: M.
- **Owner**: _TBD_
- **Issue**: [#ROAD-12](https://github.com/vaheed/kubeOP/issues/ROAD-12)

### Docs Track — Automated release governance
- **Problem**: CHANGELOG and version metadata updates rely on manual edits despite
  structured data in `internal/version/version.go`.
- **Outcome**: Automate changelog generation, version bump validation, and docs site
  publishing.
- **Scope (In)**: Add scripts/CI steps that compare `CHANGELOG.md`, `docs/RELEASES.md`,
  and version package; update `.github/workflows/ci.yml` as needed.
- **Scope (Out)**: External release notes portal.
- **Acceptance Criteria**: CI fails when version metadata drifts; docs site built on
  tag pushes; README badges update automatically.
- **Dependencies**: Existing CI pipeline.
- **Risks**: CI complexity; false positives on release detection.
- **Estimation**: M.
- **Owner**: _TBD_
- **Issue**: [#ROAD-6](https://github.com/vaheed/kubeOP/issues/ROAD-6)

## Phase: Later (12+ weeks)

### Delivery Track — Multi-cluster placement policies
- **Problem**: Project provisioning in `internal/service/projects.go` uses first-fit
  placement without policy awareness.
- **Outcome**: Policy engine for regional/label-based placement and rebalancing.
- **Scope (In)**: Extend placement logic; add policy definitions; update docs.
- **Scope (Out)**: UI visualisation.
- **Acceptance Criteria**: Policy evaluation tests, API filters, scheduler tie-ins.
- **Dependencies**: Template versioning, operator parity.
- **Risks**: Complexity, cluster drift.
- **Estimation**: L.
- **Owner**: _TBD_
- **Issue**: [#ROAD-13](https://github.com/vaheed/kubeOP/issues/ROAD-13)

### API Track — Read-only tenant portal
- **Problem**: No UI for tenants despite APIs for logs, releases, and events.
- **Outcome**: Ship read-only web portal backed by existing endpoints.
- **Scope (In)**: New frontend (e.g., VitePress or React) consuming `/v1` APIs.
- **Scope (Out)**: Write capabilities.
- **Acceptance Criteria**: Auth integration, release/event views, health indicator.
- **Dependencies**: Tenant-scoped tokens.
- **Risks**: Accessibility compliance, deployment footprint.
- **Estimation**: L.
- **Owner**: _TBD_
- **Issue**: [#ROAD-14](https://github.com/vaheed/kubeOP/issues/ROAD-14)

### Data Track — Export pipeline for analytics
- **Problem**: Operational metrics remain in Postgres/logs; no export for BI tools.
- **Outcome**: Scheduled exports to S3/object store with schema versioning.
- **Scope (In)**: Build exporters, retention policies, documentation.
- **Scope (Out)**: Managed warehouse integration.
- **Acceptance Criteria**: Successful sample export, checksum verification, docs.
- **Dependencies**: Event retention work.
- **Risks**: Data privacy, cost.
- **Estimation**: M.
- **Owner**: _TBD_
- **Issue**: [#ROAD-15](https://github.com/vaheed/kubeOP/issues/ROAD-15)

### Ops Track — Disaster recovery orchestration
- **Problem**: No automated process for region-wide failure recovery beyond manual
  backups.
- **Outcome**: Scripted failover to standby database and rehydration of cluster
  inventory.
- **Scope (In)**: Add DR scripts, document testing cadence, integrate with scheduler.
- **Scope (Out)**: Managed DR services.
- **Acceptance Criteria**: Runbook validated quarterly, tests with simulated outage.
- **Dependencies**: Backup runbooks.
- **Risks**: Complexity, false failovers.
- **Estimation**: L.
- **Owner**: _TBD_
- **Issue**: [#ROAD-16](https://github.com/vaheed/kubeOP/issues/ROAD-16)

### Security Track — Tenant-level audit trails & signing
- **Problem**: Audit logs exist only in structured application logs; no per-tenant
  signing or append-only store.
- **Outcome**: Immutable event store with signature verification for compliance.
- **Scope (In)**: Introduce append-only log (e.g., Go Merkletree); update
  `internal/service/events.go`; document retention.
- **Scope (Out)**: External compliance dashboards.
- **Acceptance Criteria**: Signed log entries, tamper detection tests, docs.
- **Dependencies**: Event retention indexes.
- **Risks**: Performance hit; cryptographic complexity.
- **Estimation**: L.
- **Owner**: _TBD_
- **Issue**: [#ROAD-17](https://github.com/vaheed/kubeOP/issues/ROAD-17)

### UX Track — Interactive status console
- **Problem**: Operators rely on log tailing and raw API calls; no consolidated
  dashboard.
- **Outcome**: Web console surfacing cluster health, releases, and events.
- **Scope (In)**: Build SPA using `/v1` endpoints; integrate metrics visualisation.
- **Scope (Out)**: Write operations.
- **Acceptance Criteria**: Read-only dashboards, RBAC, tests.
- **Dependencies**: Tenant portal groundwork.
- **Risks**: Maintenance burden, design resources.
- **Estimation**: XL.
- **Owner**: _TBD_
- **Issue**: [#ROAD-18](https://github.com/vaheed/kubeOP/issues/ROAD-18)

### Docs Track — Interactive API explorer
- **Problem**: `docs/openapi.yaml` consumed manually; no live explorer.
- **Outcome**: Embed Swagger/Redoc in docs site with version switching.
- **Scope (In)**: Docs site updates, CI build, navigation.
- **Scope (Out)**: Auth integration.
- **Acceptance Criteria**: Deployed explorer, automated build tests.
- **Dependencies**: Release governance automation.
- **Risks**: Build time increase, maintenance.
- **Estimation**: M.
- **Owner**: _TBD_
- **Issue**: [#ROAD-19](https://github.com/vaheed/kubeOP/issues/ROAD-19)

## Now / Next / Later board

| Now (0–6 w) | Next (6–12 w) | Later (12+ w) |
| --- | --- | --- |
| #ROAD-1, #ROAD-2, #ROAD-3, #ROAD-4, #ROAD-5 | #ROAD-6, #ROAD-7, #ROAD-8, #ROAD-9, #ROAD-10, #ROAD-11, #ROAD-12 | #ROAD-13, #ROAD-14, #ROAD-15, #ROAD-16, #ROAD-17, #ROAD-18, #ROAD-19 |

## Milestones & version targets

| Version | Target window | Highlights | Dependent issues |
| --- | --- | --- | --- |
| v0.12.0 | 0–6 weeks | Operator parity, release diff API, security hardening | #ROAD-1, #ROAD-2, #ROAD-3, #ROAD-4, #ROAD-5 |
| v0.13.0 | 6–12 weeks | Template versioning, tenant RBAC, observability, CLI tooling | #ROAD-6, #ROAD-7, #ROAD-8, #ROAD-9, #ROAD-10, #ROAD-11, #ROAD-12 |
| v0.14.0+ | 12+ weeks | Policy-based placement, portals, analytics exports, DR automation | #ROAD-13, #ROAD-14, #ROAD-15, #ROAD-16, #ROAD-17, #ROAD-18, #ROAD-19 |

## Readiness checklist for each deliverable

- [ ] Tests updated (`go test ./...`, `go test -count=1 ./testcase`, and relevant operator/unit suites).
- [ ] Docs updated (`README.md`, `docs/*.md`, `docs/openapi.yaml`).
- [ ] Environment/config changes captured in `docs/ENVIRONMENT.md` and samples.
- [ ] Database migrations (if any) with matching up/down files and migration tests.
- [ ] CHANGELOG entry under the target release and version bump in `internal/version` as needed.
- [ ] CI workflow adjustments to cover new tooling or scripts.
- [ ] Rollout/rollback plan documented in `docs/OPERATIONS.md` with logging hooks.

---

> Linked GitHub issues are prepared for creation using the templates in
> `.github/ISSUE_TEMPLATE/`. Apply labels matching the track (e.g., `track/delivery`),
> phase (e.g., `phase/near-term`), and size (`size/s`, `size/m`, `size/l`).
