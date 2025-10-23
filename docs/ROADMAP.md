# Roadmap

This roadmap outlines upcoming work with time boxes, acceptance criteria, and known risks. Dates assume calendar quarters.

## Q3 2025 – Multi-region health aggregation

- **Goal**: Surface regional health dashboards for fleets spanning multiple clusters.
- **Acceptance criteria**:
  - `/v1/clusters/health` accepts `?region=` and `?environment=` filters.
  - Scheduler emits per-region metrics (`kubeop_cluster_health_total{region=*}`) for Prometheus.
  - Documentation covers alerting patterns for regional outages.
- **Risks**: Additional database load when aggregating health data; ensure indexes cover `region` and `environment` columns.

## Q4 2025 – Tenant self-service portal (beta)

- **Goal**: Provide a read-only web UI for tenants to inspect projects, releases, and quotas.
- **Acceptance criteria**:
  - Static web assets served from `/portal` behind optional OIDC authentication.
  - Tenants can download kubeconfigs, view release history, and request quota increases (ticket integration).
  - Accessibility review passes WCAG 2.1 AA guidelines.
- **Risks**: Additional authentication surface; coordinate with security reviewers. UI performance depends on API pagination.

## Q1 2026 – Pluggable delivery engines

- **Goal**: Allow operators to add custom delivery backends (e.g., Terraform, Crossplane) without modifying core code.
- **Acceptance criteria**:
  - `internal/service/apps` exposes an interface for registering delivery plugins with validation and reconciliation hooks.
  - `/v1/apps/validate` and `/v1/projects/{id}/apps` accept plugin identifiers and configuration.
  - Integration tests cover at least one sample plugin implementation.
- **Risks**: Plugin API stability; need clear versioning and backwards compatibility guarantees. Potential increase in RBAC surface.

## Completed milestones

- **Q2 2025** – Documentation overhaul (this release). New IA, VitePress site, linting, and diagrams aligned with current code.
