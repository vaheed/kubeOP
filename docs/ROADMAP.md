# Roadmap

The roadmap highlights the next three quarters of work. Each item lists acceptance criteria and known risks.

## Q2 2026 – Observability expansion

- **Goal** – ship first-class telemetry integrations.
- **Acceptance criteria**
  - OpenTelemetry exporter for request traces and scheduler jobs.
  - Grafana dashboards for API latency, queue depth, and cluster health.
  - `/metrics` includes per-tenant request counters.
- **Risks**
  - Trace volume may require sampling and tuning.
  - Additional dependencies must remain optional to avoid bloating the binary.

## Q3 2026 – Tenant self-service portal

- **Goal** – provide a minimal UI for project owners.
- **Acceptance criteria**
  - Web UI for viewing cluster assignments, quotas, and deployment history.
  - OAuth2 integration for tenant logins.
  - Download links for tenant-specific kubeconfigs and logs.
- **Risks**
  - UI stack selection and accessibility compliance.
  - Ensuring RBAC parity between API and portal actions.

## Q4 2026 – Operator scalability

- **Goal** – improve `kubeop-operator` throughput and resilience.
- **Acceptance criteria**
  - Horizontal pod autoscaling guidance with leader election defaults.
  - Reconciliation batching for large namespace deployments.
  - Canary deployment strategy for operator upgrades.
- **Risks**
  - Increased complexity in the reconciliation loop may impact stability.
  - Requires extensive integration testing across Kubernetes versions.

## Continuous investments

- Security reviews and dependency updates every sprint.
- Documentation hygiene via Markdownlint, Vale, and lychee in CI.
- Community feedback triage within 48 hours.
