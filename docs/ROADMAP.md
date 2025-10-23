# Roadmap

This roadmap reflects the current kubeOP implementation across `cmd/api`,
`internal/*`, `kubeop-operator`, database migrations, and docs. Items are grouped
by phase (time horizon) and track (Delivery, API, Data, Ops, Security, UX, Docs).
Each initiative maps to concrete code surfaces with clear outcomes, scope, and
acceptance criteria.

## RICE priority ranking

RICE scoring uses estimated Reach (accounts or clusters touched per quarter),
Impact (1–3 scale), Confidence (0–1), and Effort (person-weeks, using S=1,
M=2, L=3). Higher scores should be scheduled first.

| Rank | Issue | Track | Title | Reach | Impact | Confidence | Effort | RICE Score |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | [#203](https://github.com/vaheed/kubeOP/issues/203) | Ops | Expose scheduler metrics & alerts | 70 | 2.5 | 0.65 | 1 | 113.75 |
| 2 | [#209](https://github.com/vaheed/kubeOP/issues/209) | Docs | Generate API docs from OpenAPI | 60 | 2.0 | 0.70 | 1 | 84.00 |
| 3 | [#201](https://github.com/vaheed/kubeOP/issues/201) | Delivery | Operator parity with rendered manifests | 60 | 3.0 | 0.70 | 3 | 42.00 |
| 4 | [#202](https://github.com/vaheed/kubeOP/issues/202) | API | Paginate administrative listings | 50 | 2.0 | 0.60 | 2 | 30.00 |
| 5 | [#208](https://github.com/vaheed/kubeOP/issues/208) | UX | Streaming log tails for tenants | 50 | 2.3 | 0.50 | 2 | 28.75 |
| 6 | [#205](https://github.com/vaheed/kubeOP/issues/205) | Data | Project log retention to object storage | 35 | 2.5 | 0.55 | 2 | 24.06 |
| 7 | [#204](https://github.com/vaheed/kubeOP/issues/204) | Security | Kubeconfig envelope key rotation | 40 | 3.5 | 0.50 | 3 | 23.33 |
| 8 | [#206](https://github.com/vaheed/kubeOP/issues/206) | Delivery | Template version catalog & promotion | 30 | 2.2 | 0.60 | 2 | 19.80 |
| 9 | [#207](https://github.com/vaheed/kubeOP/issues/207) | API | Region/environment health aggregation | 45 | 2.8 | 0.45 | 3 | 18.90 |

## Now / Next / Later board

| Column | Items |
| --- | --- |
| **Now** (Phase 0–6 weeks) | [#201](https://github.com/vaheed/kubeOP/issues/201), [#202](https://github.com/vaheed/kubeOP/issues/202), [#203](https://github.com/vaheed/kubeOP/issues/203) |
| **Next** (Phase 6–12 weeks) | [#204](https://github.com/vaheed/kubeOP/issues/204), [#205](https://github.com/vaheed/kubeOP/issues/205), [#206](https://github.com/vaheed/kubeOP/issues/206), [#209](https://github.com/vaheed/kubeOP/issues/209) |
| **Later** (12+ weeks) | [#207](https://github.com/vaheed/kubeOP/issues/207), [#208](https://github.com/vaheed/kubeOP/issues/208) |

## Milestone plan

| Target version | Window | Scope |
| --- | --- | --- |
| **v0.16.0** | 0–6 weeks | Deliver [#201](https://github.com/vaheed/kubeOP/issues/201), [#202](https://github.com/vaheed/kubeOP/issues/202), [#203](https://github.com/vaheed/kubeOP/issues/203). |
| **v0.17.0** | 6–12 weeks | Ship [#204](https://github.com/vaheed/kubeOP/issues/204), [#205](https://github.com/vaheed/kubeOP/issues/205), [#206](https://github.com/vaheed/kubeOP/issues/206), [#209](https://github.com/vaheed/kubeOP/issues/209). |
| **v0.18.0** | 12+ weeks | Schedule [#207](https://github.com/vaheed/kubeOP/issues/207), [#208](https://github.com/vaheed/kubeOP/issues/208). |

---

## Phase 0–6 weeks (Near-term)

### Delivery track — Operator parity with rendered manifests ([#201](https://github.com/vaheed/kubeOP/issues/201))
- **Problem**: `kubeop-operator/controllers/app_controller.go` only applies `Deployment`
  objects while `internal/service/apps.go` renders Services, Ingresses, Jobs,
  CronJobs, and config attachments. Operators must manually apply those extra
  resources, leading to drift and incomplete status updates.
- **Outcome**: The operator owns the full rendered workload set and reports
  readiness aligned with the resources created by the API layer.
- **Scope**:
  - **In**: Extend reconciler to render Services/Ingress/Jobs/CronJobs, capture
    status into `AppStatus`, add controller-runtime unit tests, update
    `docs/OPERATIONS.md` and `docs/API.md` responses.
  - **Out**: Blue/green rollout strategies, canary automation, Helm templating
    changes.
- **Acceptance Criteria**:
  - [ ] Operator applies all objects returned by `internal/service.apps.RenderedObjects`.
  - [ ] `/v1/projects/{id}/apps/{appId}` reflects Service endpoints and ingress
        hosts without manual patching.
  - [ ] `testcase/service_status_test.go` gains assertions for new resource kinds.
  - [ ] Docs updated describing operator responsibilities.
- **Dependencies**: controller-runtime client upgrades (if required for
  additional object types).
- **Risks**: Wider reconcile surface could introduce accidental deletes;
  mitigate with dry-run diff tests.
- **Estimation**: L (3 person-weeks).
- **Owner**: TBD (Staff PM/Tech Lead).

### API track — Paginate administrative listings ([#202](https://github.com/vaheed/kubeOP/issues/202))
- **Problem**: `internal/api/router.go` exposes cluster, user, and project list
  endpoints without robust pagination or filtering; service calls such as
  `CheckCluster` fetch all clusters to determine names (`ListClusters`).
- **Outcome**: Administrative endpoints accept `limit`, `offset`, and filter
  parameters (environment, region, owner). Service methods avoid loading all
  rows into memory for each request.
- **Scope**:
  - **In**: Update `listClusters`, `listUsers`, `CheckCluster`, and store queries;
    add indexes if needed; update `docs/API.md` and `docs/openapi.yaml`.
  - **Out**: Search endpoints, UI portal.
- **Acceptance Criteria**:
  - [ ] API accepts pagination params with bounds enforcement and tests in
        `testcase/api_projects_list_routes_test.go`.
  - [ ] `CheckCluster` fetches metadata without scanning every cluster.
  - [ ] OpenAPI schema documents new query parameters.
- **Dependencies**: None beyond existing Postgres schema.
- **Risks**: Query regressions if indexes missing; validate with explain plans.
- **Estimation**: M (2 person-weeks).
- **Owner**: TBD.

### Data track — *No near-term items scheduled*
- Maintain focus on Delivery/API/Ops before expanding data initiatives.

### Ops track — Expose scheduler metrics & alerts ([#203](https://github.com/vaheed/kubeOP/issues/203))
- **Problem**: `internal/service/healthscheduler.go` emits logs only; Prometheus
  `/metrics` exposes readiness counters only (`internal/metrics/readyz.go`).
  Operators lack numeric time-series for health runs.
- **Outcome**: Scheduler publishes metrics such as `kubeop_cluster_health_total`
  with labels (healthy/unhealthy/region) and histogram timings; docs describe
  alerting.
- **Scope**:
  - **In**: Add metrics registry, instrument `TickWithSummary`, extend
    `/metrics` exposition tests, update `docs/OPERATIONS.md`.
  - **Out**: External alertmanager configuration or dashboards.
- **Acceptance Criteria**:
  - [ ] Prometheus metrics include per-run cluster counts and durations.
  - [ ] `testcase/service_scheduler_test.go` asserts metric updates.
  - [ ] Operations guide documents sample alert rules.
- **Dependencies**: None.
- **Risks**: Over-labeling metrics; keep cardinality bounded (region/env only).
- **Estimation**: S (1 person-week).
- **Owner**: TBD.

### Security track — *No near-term items scheduled*
- Preparing groundwork for key rotation in the next phase.

### UX track — *No near-term items scheduled*
- Streaming log UX deferred to later phase.

### Docs track — *No near-term items scheduled*
- Awaiting automation work in the next phase.

---

## Phase 6–12 weeks (Mid-term)

### Delivery track — Template version catalog & promotion ([#206](https://github.com/vaheed/kubeOP/issues/206))
- **Problem**: `internal/service/templates.go` stores single versions only.
  Updating a template requires manual replacements without history, blocking
  promotion flows.
- **Outcome**: Templates gain version metadata and promotion workflow (draft →
  published) with API support for listing history.
- **Scope**:
  - **In**: Extend migrations (`internal/store/migrations`) for template
    revisions, update service and API handlers, add tests, update docs.
  - **Out**: Git-backed template storage or UI.
- **Acceptance Criteria**:
  - [ ] New schema tables for template revisions with sequential versions.
  - [ ] `/v1/templates` returns version history with filters in OpenAPI.
  - [ ] Regression tests cover promotion/demotion flows.
- **Dependencies**: Database migration version 0023+, docs update.
- **Risks**: Data migration complexity; provide migration script for existing
  templates.
- **Estimation**: M (2 person-weeks).
- **Owner**: TBD.

### API track — *No mid-term items beyond pagination* (focus shifts to later aggregation).

### Data track — Project log retention to object storage ([#205](https://github.com/vaheed/kubeOP/issues/205))
- **Problem**: `internal/logging/files.go` writes rotated files locally; there is
  no archival pipeline for long-term retention or disaster recovery.
- **Outcome**: Introduce pluggable log sinks (S3/GCS) with background upload and
  lifecycle configuration.
- **Scope**:
  - **In**: Extend FileManager to stream to object storage, add configuration
    (`internal/config.Config`), document operations, add tests.
  - **Out**: Real-time log search UI.
- **Acceptance Criteria**:
  - [ ] Config supports enabling an object storage sink with credentials.
  - [ ] Integration tests stub uploads; docs describe retention policies.
  - [ ] Local rotation continues to function when sink disabled.
- **Dependencies**: Credential management in `internal/service/credentials.go`.
- **Risks**: Secret handling for storage keys; rely on env vars/secret stores.
- **Estimation**: M (2 person-weeks).
- **Owner**: TBD.

### Ops track — *No mid-term additions* (metrics work finishes prior phase).

### Security track — Kubeconfig envelope key rotation ([#204](https://github.com/vaheed/kubeOP/issues/204))
- **Problem**: `internal/service/kubeconfigs.go` and `crypto` derive a fixed key
  from `KCFG_ENCRYPTION_KEY`; there is no rotation path and encrypted records in
  Postgres cannot be re-encrypted without downtime.
- **Outcome**: Introduce dual-key support with migration tooling and API to
  rotate encryption keys while keeping kubeconfigs accessible.
- **Scope**:
  - **In**: Extend config to accept primary/secondary keys, add migration job
    to re-encrypt secrets, update docs/CHANGELOG, add tests.
  - **Out**: Automatic key rotation scheduling.
- **Acceptance Criteria**:
  - [ ] Service can decrypt with old key and re-encrypt with new key without
        downtime (tests in `testcase/service_kubeconfig_helpers_test.go`).
  - [ ] `/v1/kubeconfigs/rotate` triggers re-encryption when new key provided.
  - [ ] Security docs outline rotation steps.
- **Dependencies**: Potential downtime planning; coordinate with Ops.
- **Risks**: Data loss if rotation fails; ensure transactional rollback.
- **Estimation**: L (3 person-weeks).
- **Owner**: TBD.

### UX track — *No mid-term items scheduled*
- Streaming UX deferred to later phase once backend pagination lands.

### Docs track — Generate API docs from OpenAPI ([#209](https://github.com/vaheed/kubeOP/issues/209))
- **Problem**: `docs/API.md` and `docs/openapi.yaml` drift without automation;
  contributors update one but not the other.
- **Outcome**: CI step generates API reference tables from the OpenAPI spec and
  validates docs during PRs.
- **Scope**:
  - **In**: Add npm script to render markdown from OpenAPI, update CI to run it,
    document workflow in `docs/STYLEGUIDE.md`.
  - **Out**: Public hosted reference beyond docs site.
- **Acceptance Criteria**:
  - [ ] `npm run docs:api` (new) regenerates API tables from `docs/openapi.yaml`.
  - [ ] CI fails when docs are out of sync.
  - [ ] Contributors updated guidance in README and STYLEGUIDE.
- **Dependencies**: Node tooling (e.g., `widdershins` or custom script).
- **Risks**: Tooling drift; pin npm dependencies in `package.json`.
- **Estimation**: S (1 person-week).
- **Owner**: TBD.

---

## Phase 12+ weeks (Later)

### Delivery track — *No later items beyond template work* (monitor adoption).

### API track — Region/environment health aggregation ([#207](https://github.com/vaheed/kubeOP/issues/207))
- **Problem**: `/v1/clusters/health` returns per-cluster status only. Operators
  need aggregated views (by region/environment) leveraging `cluster_status`
  history in Postgres.
- **Outcome**: API exposes aggregated metrics and historical summaries for
  dashboards.
- **Scope**:
  - **In**: Add summary queries, extend API responses, update docs/OPERATIONS.
  - **Out**: Real-time dashboards.
- **Acceptance Criteria**:
  - [ ] New endpoints or query params return aggregated counts by region/env.
  - [ ] Tests cover summary calculations.
  - [ ] Metrics exported for aggregated health.
- **Dependencies**: Scheduler metrics (GH-203) must land first.
- **Risks**: Query performance; may need materialized views.
- **Estimation**: L (3 person-weeks).
- **Owner**: TBD.

### Data track — *No later additions currently planned*.

### Ops track — *No later additions currently planned*.

### Security track — *No later additions currently planned*.

### UX track — Streaming log tails for tenants ([#208](https://github.com/vaheed/kubeOP/issues/208))
- **Problem**: `internal/api/projects.go` only serves static log tails or entire
  files; no follow/streaming support. Tenants lack real-time feedback during
  deployments.
- **Outcome**: Provide chunked/SSE log streaming with rate limiting and tests.
- **Scope**:
  - **In**: Add `follow` query parameter with SSE or WebSocket support, update
    docs, add client examples in `docs/examples/curl`.
  - **Out**: Web UI for logs.
- **Acceptance Criteria**:
  - [ ] Log endpoint streams updates when `follow=true` with backpressure.
  - [ ] Tests cover SSE/WebSocket behaviours.
  - [ ] Docs and samples updated with usage guidance.
- **Dependencies**: Ensure log retention (GH-205) handles streaming writes.
- **Risks**: Resource usage from long-lived connections; enforce limits.
- **Estimation**: M (2 person-weeks).
- **Owner**: TBD.

### Docs track — *No later additions currently planned*.

---

## Deprecations & migration notes

- GH-201 will deprecate manual application of Services/Ingresses; operators must
  ensure the kubeop-operator has cluster-wide permissions for those objects.
- GH-204 introduces dual-key encryption. Deployments must supply both primary
  and secondary keys during rotation windows.

## Readiness checklist (apply per roadmap item before marking Done)

- [ ] Tests updated (`go test ./...`, `go test -count=1 ./testcase`, operator
      tests where relevant).
- [ ] Documentation refreshed (`README.md`, relevant `docs/*.md`, OpenAPI,
      release notes).
- [ ] Environment/config changes documented (`docs/ENVIRONMENT.md`, samples).
- [ ] Database migrations reviewed and reversible; recovery steps documented.
- [ ] CHANGELOG entry under `[Unreleased]` with version bump when behaviour
      changes.
- [ ] Rollout plan captured (maintenance mode, operator upgrades, backouts).
- [ ] GitHub issue labels (`track:*`, `phase:*`, `size:*`) reflect final scope.
