---
outline: deep
---

# Stage 00–04 Audit Matrix

| Stage | Goal / Deliverable | Code Path(s) | Tests | Documentation |
| --- | --- | --- | --- | --- |
| 00 | Manager API with cluster/tenant/project/app CRUD, JWT auth, Postgres storage. | `internal/server/api.go`, `internal/models/` | `internal/server/api_test.go`, `internal/models/models_test.go` | [API Reference](./api-reference.md), [Bootstrap Guide](./bootstrap-guide.md) |
| 01 | Operator reconciliation for namespaces, ResourceQuota, NetworkPolicy, delivery kinds. | `internal/operator/controllers.go` | `internal/operator/controllers_test.go` | [Delivery Workflows](./delivery.md), [Operations](./operations.md) |
| 02 | Admission webhooks enforcing ownership, quotas, registry allowlists, egress baselines. | `internal/admission/` | `internal/admission/server_test.go` | [Operations](./operations.md) |
| 03 | Multi-source delivery (Image/Git/Helm/Raw), hooks, revision store, rollout/rollback automation. | `internal/operator/controllers.go` | `e2e/run.sh` (rollout/rollback, Git/Helm apps) | [Delivery Workflows](./delivery.md), [Bootstrap Guide](./bootstrap-guide.md) |
| 04 | Usage reporting, invoice export, analytics, docs & diagrams, CI enforcement. | `internal/server/api.go` (usage/invoice/analytics handlers), `e2e/run.sh`, `Makefile`, `docs/` | `internal/server/api_test.go` (UsageInvoiceAnalytics), `make test-e2e` artifacts (`artifacts/usage.json`, etc.) | [API Reference](./api-reference.md), [Getting Started](./getting-started.md), [Production Hardening](./production-hardening.md), diagrams in `docs/diagrams/` |

Refer to `CHANGELOG.md` for chronological release notes. The automated `make test-e2e` run asserts each stage end-to-end, producing artifacts consumed by review.
