# Getting Started

Prerequisites: Kubernetes 1.26+, kubectl, Helm 3, Docker (for Kind/Compose).

Local (Kind + Compose):

- Create cluster: make kind-up
- Bootstrap operator/admission: bash e2e/bootstrap.sh
- Start Manager + Postgres: docker compose up -d db manager
- Verify: curl -sf localhost:18080/healthz
