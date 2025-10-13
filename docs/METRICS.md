Metrics

Prometheus endpoint

- GET `/metrics` — Exposes Prometheus metrics for scraping.

Exported metrics (initial)

- `readyz_failures_total{reason="..."}` (counter): readiness probe failures grouped by reason (`service_missing`, `health_check_failed`, etc.).
- `kubeop_tenant_count` (gauge): number of projects/tenants.
- `kubeop_reconcile_duration_seconds` (histogram): reconciliation durations.
- `kubeop_quota_usage_percent{resource=...}` (gauge): percentage of quota used, when available.
- `kubeop_certificate_expiry_days` (gauge): cert-manager certificate remaining days (future).

Scrape example (Prometheus)

- Add a job:
  - `- job_name: kubeop\n  static_configs:\n  - targets: ['kubeop:8080']`

Alerts (ideas)

- Readiness regression: `increase(readyz_failures_total[5m]) > 3` should page operators; wire this to Grafana/Alertmanager alongside the structured `readyz` WARN logs.
- Tenant quota over 85% for CPU or Memory.
- Cert expiry less than 14 days.

