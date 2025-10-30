---
outline: deep
---

# Production Hardening

This guide summarises deployment choices and guardrails for running kubeOP in mission-critical environments.

## Capacity planning

| Component | Recommended replicas | CPU / Memory requests | Notes |
| --- | --- | --- | --- |
| Manager API | 2 | 250m / 512Mi | Stateless (sticky sessions not required). Behind a load balancer. |
| Operator | 2 | 200m / 256Mi | Use leader election (enabled by controller-runtime). |
| Admission webhooks | 2 | 200m / 256Mi | TLS served via Helm chart. PDB ensures at least one replica available. |
| PostgreSQL | Managed service or HA StatefulSet | Follow provider guidance | Enable PITR and daily snapshots. |

## High availability & PDBs

- Enable PodDisruptionBudgets:
  ```bash
  kubectl -n kubeop-system apply -f - <<'YAML'
  apiVersion: policy/v1
  kind: PodDisruptionBudget
  metadata:
    name: kubeop-manager
  spec:
    minAvailable: 1
    selector:
      matchLabels:
        app: kubeop-manager
  YAML
  ```
- Admission PDB is shipped in the Helm chart (`values.admission.pdb.enabled=true`).

## Probes & health checks

- Manager exposes `/healthz` and `/metrics` on the main process (`KUBEOP_HTTP_ADDR`).
- Operator exposes separate HTTP endpoints (see `internal/operator/manager.go`).
- Admission container uses HTTPS health probes; configure `failureThreshold` to 3 with 5s periods.

## Logging & tracing

- Structured logs include context (request ID, cluster/app IDs). Forward to a centralized aggregator with log levels set via `LOG_LEVEL` (default `info`).
- For tracing, sidecar instrumentation (e.g., OpenTelemetry Collector) can wrap the manager deployment by injecting OTLP exporters.

## Metrics & alerting

- Scrape `/metrics` using Prometheus. Key metrics:
  - `http_request_duration_seconds` and `_errors_total` for the manager.
  - `controller_runtime_reconcile_total` labelled by GVK for operator health.
  - `admission_requests_total` from the webhook deployment.
- Alert thresholds:
  - Manager error rate >1% over 5 minutes.
  - Reconcile errors sustained for >3 minutes.
  - Admission replica count < desired.

## Secrets management

- Store `KUBEOP_DB_URL`, `KUBEOP_KMS_MASTER_KEY`, `KUBEOP_JWT_SIGNING_KEY`, `KUBEOP_IMAGE_ALLOWLIST`, and `KUBEOP_EGRESS_BASELINE` in your secrets manager (e.g., AWS Secrets Manager) and mount into Kubernetes Secrets.
- Rotate keys quarterly; ensure new JWT signing keys are distributed to automation before flipping env vars.

## Disaster recovery

1. Restore Postgres from the most recent snapshot (verify WAL/PITR replay).
2. Recreate Kubernetes namespaces (`kubeop-system`, tenant namespaces) using IaC.
3. Reapply CRDs and operator manifests (`make operator-up` or Helm chart).
4. Replay cluster registrations via `/v1/clusters` (manager will bootstrap components automatically).
5. Re-run `make test-e2e` in a staging cluster after recovery to validate invariants.

## Service level objectives

| SLI | Target |
| --- | --- |
| Manager API availability | 99.9% monthly |
| Admission webhook availability | 99.9% monthly |
| Reconciliation latency | < 60s P95 from spec change to workload updated |
| Invoice export correctness | 100% (validated by comparing `/v1/usage/snapshot` vs `/v1/invoices/{tenant}`) |

Document SLO breaches using your incident tooling and feed learnings back into resource tuning.
