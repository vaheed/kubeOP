Roadmap

Phase 1 (This PR)

- Out-of-cluster API service with env-driven config
- Postgres store + embedded migrations
- Health, readiness, version endpoints
- Cluster registry (encrypt kubeconfigs) and users
- Dockerfile + docker-compose + CI workflows

Phase 2

- Tenant/project modeling and per-tenant API tokens
- Namespace-scoped kubeconfig generation for end-users
- Multi-cluster client pooling with TTL and metrics
- Basic app lifecycle (register/update/remove) stubs

Phase 3

- Observability: metrics, tracing, audit logs
- RBAC and authorization policies
- Backup/restore tooling and lifecycle hooks
- Optional pluggable storage and secrets managers

w