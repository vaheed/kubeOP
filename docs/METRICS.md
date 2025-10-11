Metrics

Prometheus endpoint

- GET `/metrics` — Exposes Prometheus metrics for scraping.

Exported metrics (initial)

- `kubeop_tenant_count` (gauge): number of projects/tenants.
- `kubeop_reconcile_duration_seconds` (histogram): reconciliation durations.
- `kubeop_quota_usage_percent{resource=...}` (gauge): percentage of quota used, when available.
- `kubeop_certificate_expiry_days` (gauge): cert-manager certificate remaining days (future).

Scrape example (Prometheus)

- Add a job:
  - `- job_name: kubeop\n  static_configs:\n  - targets: ['kubeop:8080']`

Alerts (ideas)

- Tenant quota over 85% for CPU or Memory.
- Cert expiry less than 14 days.

