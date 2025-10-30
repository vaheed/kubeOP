---
outline: deep
---

# Phase A Roadmap (00–04)

Phase A establishes kubeOP's foundation: a complete local harness, the management
API and database, multi-tenant CRDs with operator reconciliation, admission
webhooks, and delivery/analytics workflows. Each phase builds on the previous
one and is reflected in source control, manifests, tests, and documentation.

## 00 — Kind E2E Harness

- **Goal**: Reproduce the full platform locally with Kind + Docker Compose and
  exercise an end-to-end tenant/project/app workflow.
- **Code & assets**: `e2e/kind-config.yaml`, `e2e/bootstrap.sh`, `e2e/run.sh`,
  `Makefile` targets (`kind-up`, `platform-up`, `manager-up`, `test-e2e`).
- **Tests**: `tests/e2e/*.go` collect the smoke-test summary artifacts.
- **Docs**: [Getting Started](./getting-started.md#local-evaluation-kind--compose--helm)
  walks through the commands and expected artifacts.

## 01 — Manager API & Database

- **Goal**: CRUD for clusters/tenants/projects/apps backed by PostgreSQL with
  JWT auth, bootstrap integration, and usage/invoice APIs.
- **Code**: `internal/server/api.go`, `internal/models/*.go`, `internal/db/*`,
  `cmd/manager` entrypoint.
- **Tests**: `internal/server/api_test.go`, `internal/models/models_test.go`.
- **Docs**: [API Reference](./api-reference.md), [Bootstrap Guide](./bootstrap-guide.md).
- **Manifests**: `deploy/k8s/manager/migrations-job.yaml` for day-2 migrations
  using the manager image's new `migrate` mode.

## 02 — Operator: Tenant / Project / App

- **Goal**: Reconcile kubeOP CRDs, namespaces, ResourceQuota, and NetworkPolicy
  with label/finalizer ownership enforced across the hierarchy.
- **Code**: `internal/operator/controllers.go` (Tenant/Project/App/Policy/Registry/DNS/Certificate).
- **Tests**: `internal/operator/controllers_test.go`.
- **Docs**: [Operations](./operations.md#tenant-project-and-app-reconciliation)
  and embedded diagrams.
- **Manifests**: `deploy/k8s/crds/*.yaml`, `deploy/k8s/operator/*.yaml`, Helm
  chart under `charts/kubeop-operator/`.

## 03 — Admission Webhooks

- **Goal**: Mutate and validate CRDs to enforce namespaces, quotas, registry
  allowlists, and egress baselines with HA deployment assets.
- **Code**: `internal/admission/*.go` server and metrics.
- **Tests**: `internal/admission/server_test.go`, `internal/admission/metrics_test.go`.
- **Docs**: [Operations](./operations.md#admission-controls),
  [Audit Matrix](./audit-matrix.md).
- **Manifests**: `deploy/k8s/admission/*.yaml`, Helm templates for admission
  service/PDB/webhooks.

## 04 — Delivery Controller & Analytics

- **Goal**: Multi-source delivery (Image/Git/Helm/Raw), revision history,
  hooks, DNS/TLS orchestration, usage/invoice/analytics reporting, diagrams,
  and CI wiring.
- **Code**: `internal/operator/controllers.go` (delivery logic),
  `internal/server/api.go` (usage, invoice, analytics), `docs/diagrams/*.svg`.
- **Tests**: `e2e/run.sh` scenario (Git/Helm apps, DNS/TLS),
  `internal/server/api_test.go` (usage/invoice/analytics coverage).
- **Docs**: [Delivery Workflows](./delivery.md), [Operations](./operations.md),
  [Production Hardening](./production-hardening.md).
- **CI/CD**: `.github/workflows/ci.yaml` runs lint/tests/build, Kind E2E,
  chart packaging, and multi-arch image builds.

Use this roadmap with the [Stage 00–04 Audit Matrix](./audit-matrix.md) to
cross-reference implementation, tests, and documentation for every Phase A
milestone.
