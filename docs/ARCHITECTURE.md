# Architecture

kubeOP keeps the control plane outside your clusters. This document explains the components, data flow, and operational
boundaries so you can reason about upgrades and integrations.

## Component overview

```mermaid
flowchart TD
  subgraph Clients
    CLI["CLI / scripts"]
    Portal["Internal portals"]
    Automation["CI pipelines"]
  end
  subgraph ControlPlane
    API["API server\n(cmd/api)"]
    Service["Service layer\n(internal/service)"]
    Store[("PostgreSQL\n(internal/store)")]
    Scheduler["Health scheduler"]
    Logs[("Project logs\nlogs/<id>")]
  end
  subgraph Clusters
    Operator["kubeop-operator"]
    KubeAPI[("Kubernetes API")]
  end
  Clients -->|JWT/HTTPS| API
  API --> Service
  Service --> Store
  Service --> Logs
  Service -->|controller-runtime| Operator
  Operator --> KubeAPI
  Scheduler --> Store
  Scheduler --> API
```

The API (Go + `chi`) performs request authentication, validation, and auditing. The service layer orchestrates store queries,
Kubernetes interactions, and log writing. The scheduler continuously refreshes cluster health snapshots and persists them for API
consumers. Within each managed cluster, the `kubeop-operator` reconciles `App` CRDs rendered by the service layer.

The Mermaid source and exported diagrams live in `docs/media/`.

## Request lifecycle

```mermaid
sequenceDiagram
  participant Client
  participant API as internal/api
  participant Service as internal/service
  participant Store as PostgreSQL
  participant Operator as kubeop-operator
  participant Cluster as Kubernetes

  Client->>API: POST /v1/projects {userId, clusterId, name}
  API->>Service: CreateProject
  Service->>Store: Persist project metadata
  Service->>Cluster: Create namespace, quotas, and RBAC
  Service-->>API: ProjectCreateResponse
  API-->>Client: 201 Created

  Client->>API: POST /v1/projects/{id}/apps {image/helm/git/oci}
  API->>Service: DeployApp
  Service->>Operator: Render CRD & hand off
  Operator->>Cluster: Reconcile manifests
  Operator-->>Service: Status updates
  Service-->>API: Deployment summary
  API-->>Client: 201 Created
```

## Data storage

- **PostgreSQL** – clusters, users, projects, apps, credential stores, templates, events, and scheduler snapshots.
- **Filesystem (`logs/`)** – append-only per-project logs, application delivery records, and user-accessible archives.
- **OpenAPI schema** – published at [`docs/openapi.yaml`](openapi.yaml) and surfaced via `/v1/openapi`.

## Background jobs

- **Cluster health scheduler** (`internal/service/healthscheduler.go`) runs at `CLUSTER_HEALTH_INTERVAL_SECONDS` and records the
  latest probe results. Responses surface through `/v1/clusters/{id}/status`.
- **Log maintenance** (`internal/service/logs.go`) rotates project logs and ensures directories exist at startup.

## Extensibility points

- **Delivery engines** – `internal/service/apps.go` supports container images, Helm charts, Git repos, and OCI bundles. New
  delivery types plug into this module.
- **DNS providers** – `internal/service/dns/*.go` implements Cloudflare and PowerDNS integrations controlled by environment
  variables.
- **Operator hooks** – `kubeop-operator/` uses controller-runtime; add new reconcilers or CRDs as required.

## Trust boundaries

- kubeOP never stores plaintext kubeconfigs; they are encrypted with `KCFG_ENCRYPTION_KEY` and decrypted only in memory.
- All admin actions require JWTs signed with `ADMIN_JWT_SECRET` unless `DISABLE_AUTH=true` (development only).
- The API can run outside Kubernetes; ensure outbound network access to cluster API servers and inbound access is restricted via
  your chosen ingress/load balancer.
