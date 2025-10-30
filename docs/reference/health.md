---
outline: deep
---

# Health/Ready/Version/Metrics

Every service exposes:

- `/healthz` – internal probes pass
- `/readyz` – dependencies ready (DB/KMS/clients)
- `/version` – `{ service, version, gitCommit, buildDate }`
- `/metrics` – Prometheus metrics (go/process + domain)

Helm adds Service/ServiceMonitor for operator metrics.

