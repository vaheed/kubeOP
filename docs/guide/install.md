---
outline: deep
---

# Install

- Kind cluster: `make kind-up && bash e2e/bootstrap.sh`
- Manager API (Compose): `docker compose up -d db manager`
- Verify: `curl -s localhost:18080/healthz && kubectl -n kubeop-system get deploy/kubeop-operator`
- E2E artifacts: `make test-e2e` stores logs, metrics, DB dumps, and JSON step traces in `artifacts/`

