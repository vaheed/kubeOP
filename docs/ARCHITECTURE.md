# Architecture

kubeOP separates the control plane from managed clusters. The API server coordinates persistence, scheduling, and delivery, while
a lightweight operator inside each cluster reconciles rendered applications.

## Component overview

::: include docs/_snippets/diagram-architecture.md
:::

The architecture centres around four packages:

- **`cmd/api`** – Entrypoint that loads configuration, initialises logging, runs database migrations, and exposes the HTTP server.
- **`internal/service`** – Business logic for clusters, users, projects, apps, credentials, DNS automation, and maintenance mode.
- **`internal/store`** – PostgreSQL wrapper with embedded migrations and typed repositories for clusters, projects, and releases.
- **`kubeop-operator`** – Controller-runtime manager that reconciles the `App` custom resource and updates status back to the API.

Supporting packages provide typed Kubernetes clients (`internal/kube`), logging helpers (`internal/logging`), and delivery engines
for container, Helm, Git, and OCI sources (`internal/service/apps.go`).

## Request flow

::: include docs/_snippets/diagram-delivery-flow.md
:::

1. Admin clients send REST requests to `/v1/*` endpoints with an administrator JWT.
2. The router enforces maintenance mode and forwards requests to the service layer.
3. The service persists state in PostgreSQL, encrypts sensitive data, and talks to managed clusters via cached Kubernetes clients.
4. Deployment requests render manifests, ensure DNS/ingress policies, and record release metadata and SBOM fingerprints.
5. The kubeop-operator reconciles the App CRD, reporting pod readiness, ingress hosts, and conditions back to the API.
6. Responses include deterministic labels, delivery metadata, and quota usage so automation can react.

## Scheduler and health checks

::: include docs/_snippets/diagram-scheduler.md
:::

- The cluster health scheduler runs inside `cmd/api`, polling on `CLUSTER_HEALTH_INTERVAL_SECONDS`.
- Each tick lists registered clusters, probes readiness, and records structured logs plus database snapshots.
- Failures include cluster ID, name, and error string so operators can alert on repeated outages.

## Logging, metrics, and events

- Structured logs include request IDs and actor metadata, making it easy to trace calls across services.
- `/metrics` exposes Prometheus-format metrics (HTTP latency, scheduler timings) without authentication.
- `/v1/projects/{id}/logs` and `/v1/projects/{id}/apps/{appId}/logs` proxy log streams from managed clusters.
- `/v1/events/ingest` accepts batched Kubernetes events when `EVENT_BRIDGE_ENABLED=true` for out-of-band observability.

## Operator responsibilities

- Install the `kubeop-operator` in every managed cluster referenced by kubeOP.
- The operator reconciles rendered Deployments, Services, Ingresses, Jobs, and CronJobs created by the API.
- Status updates surface desired/ready replicas, conditions, and ingress hosts back to kubeOP for API responses and release history.

## Where to go next

- [API](API.md) – Endpoint catalogue and payload examples.
- [Operations](OPERATIONS.md) – Backups, upgrades, maintenance mode, and observability hooks.
- [SECURITY](SECURITY.md) – Threat model and hardening guidance.
