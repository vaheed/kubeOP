# Architecture

kubeOP keeps the control plane outside managed clusters. It authenticates every admin call, persists state in PostgreSQL, and reconciles Kubernetes resources on demand. This page outlines the core concepts, data flows, and diagrams that describe the system.

## Domain concepts

| Concept | Description |
| --- | --- |
| Tenant | Logical owner of namespaces and projects. Tenants are represented by users bootstrapped through `/v1/users/bootstrap`, which provisions dedicated namespaces and kubeconfigs. |
| Cluster | Registered Kubernetes target (stored via `internal/store/clusters.go`). kubeOP stores encrypted kubeconfigs and uses them to create controller-runtime clients per request. |
| Project | Application workspace tied to a user namespace (`internal/store/projects.go`). Projects receive quotas, limit ranges, and managed annotations and can be suspended or deleted. |
| App | Deployed workload associated with a project. The service layer renders Deployments, Services, Ingresses, Jobs, or raw manifests depending on the payload (`internal/service/apps.go`). |
| Watcher | Out-of-cluster deployment that streams filtered Kubernetes resource changes back to kubeOP via the batching sink (`cmd/kubeop-watcher`, `internal/watch`). |
| Quota profile | Default ResourceQuota and LimitRange values derived from configuration (`internal/service/quota.go`, `internal/config/config.go`). |

## High-level architecture

```mermaid
flowchart TD
    subgraph Clients
        CLI["curl / CI"]
        Integrations["Internal portals"]
    end
    subgraph ControlPlane
        API["internal/api<br/>HTTP handlers"]
        MW["Admin auth<br/>+ audit middleware"]
        Service["internal/service<br/>business logic"]
        Store[("PostgreSQL<br/>(internal/store)")]
        Scheduler["Cluster health<br/>scheduler"]
        LogFS["logs/<project> and<br/>events JSONL"]
    end
    subgraph External
        Clusters[("Managed Kubernetes<br/>clusters")]
        WatcherDeploy["watcherdeploy<br/>manifest orchestrator"]
        WatcherProc["kubeop-watcher<br/>Deployment"]
        Sink["internal/sink<br/>HTTPS batches"]
    end
    CLI -->|JWT/HTTPS| MW --> API
    Integrations --> MW
    API --> Service
    Service --> Store
    Service -->|controller-runtime| Clusters
    Service --> LogFS
    Service -->|auto deploy manifests| WatcherDeploy --> WatcherProc --> Sink -->|POST /v1/events/ingest (planned)| API
    Scheduler --> Store
    Scheduler --> API
```

## Request lifecycle

```mermaid
sequenceDiagram
    participant Client
    participant API as internal/api
    participant Service as internal/service
    participant Store as PostgreSQL
    participant Cluster as Kubernetes API

    Client->>API: POST /v1/projects {userId, clusterId, name}
    API->>Service: CreateProject(ctx, input)
    Service->>Store: Create project row, ensure namespace
    Service->>Cluster: Apply quota, limit range, labels
    Service->>Store: Record kubeconfig binding & events
    Service-->>API: ProjectCreateOutput (ids, namespace)
    API-->>Client: 201 Created
    Client->>API: POST /v1/projects/{id}/apps {image/helm/manifests}
    API->>Service: DeployApp(ctx, input)
    Service->>Cluster: Render manifests, apply via controller-runtime
    Service->>LogFS: Append deploy audit log
    Service-->>API: AppDeployOutput (appId, service info)
    API-->>Client: 201 Created
    Client->>API: GET /v1/projects/{id}/apps/{appId}
    API->>Service: GetProjectApp
    Service->>Cluster: Inspect Deployment, Service, Ingress, Pods
    Service-->>API: AppStatus (desired/ready, ingress hosts)
    API-->>Client: 200 OK
```

## Watcher sync pipeline

```mermaid
flowchart LR
    subgraph Control Plane
        Register["Cluster registration"]
        Token["Generate watcher JWT"]
        WDDeployer["watcherdeploy.Ensure"]
        EventsAPI["POST /v1/events/ingest<br/>(planned)"]
    end
    subgraph Managed Cluster
        Namespace["kubeop-system"]
        WatcherSA["ServiceAccount + RBAC"]
        WatcherSecret["Token + config Secret"]
        WatcherPVC["PVC for informer state"]
        WatcherDeployment["kubeop-watcher"]
        ClusterAPI[("Kubernetes API")]
    end
    subgraph Watcher Runtime
        Manager["internal/watch.Manager"]
        Sink["internal/sink.Sink"]
        Store["state.Store"]
    end

    Register -->|WatcherAutoDeploy=true| Token --> WDDeployer
    WDDeployer --> Namespace
    WDDeployer --> WatcherSA
    WDDeployer --> WatcherSecret
    WDDeployer --> WatcherPVC
    WDDeployer --> WatcherDeployment
    WatcherDeployment --> Manager
    WatcherDeployment --> ClusterAPI
    Manager -->|List/Watch| ClusterAPI
    Manager --> Store
    Manager -->|Summaries + dedup key| Sink
    Sink -->|Batch, gzip, retry| EventsAPI
```

## Deployment topology

```mermaid
flowchart TB
    subgraph Operator Network
        ControlPlane[Control plane VM / container]
        Postgres[(PostgreSQL 14+)]
        ObjectStorage[(Optional log archive)]
    end
    subgraph Cluster A
        WatcherA[kubeop-watcher Deployment]
        TenantNamespacesA[(Tenant namespaces)]
    end
    subgraph Cluster B
        WatcherB[kubeop-watcher Deployment]
        TenantNamespacesB[(Tenant namespaces)]
    end

    ControlPlane -->|TCP 5432| Postgres
    ControlPlane -->|HTTPS :8080| Internet
    ControlPlane -->|kubectl via kubeconfig| TenantNamespacesA
    ControlPlane --> TenantNamespacesB
    ControlPlane -->|HTTPS ingest| WatcherA
    ControlPlane --> WatcherB
    WatcherA -->|Outbound HTTPS| ControlPlane
    WatcherB -->|Outbound HTTPS| ControlPlane
    ControlPlane -->|Log rotation| ObjectStorage
```

## Data and control flows

### API and middleware

- `internal/api/router.go` registers health/readiness/version endpoints, wraps all `/v1` routes with `AdminAuthMiddleware`, and emits audit/structured logs. Health checks defer to a pluggable interface so the scheduler and store dependencies surface errors quickly.

### Service layer

- `internal/service/apps.go` renders workloads from multiple input types (image, Helm chart, raw manifests), validates ports and domains, and reconciles Kubernetes resources via controller-runtime clients.
- `internal/service/configs.go`, `secrets.go`, and `events.go` manage ConfigMap/Secret lifecycles and persist project events through the store.
- `internal/service/kubeconfigs.go` encrypts kubeconfigs, rotates tokens, and maintains per-user/project bindings with namespace-scoped RBAC.

### Persistence and logs

- `internal/store` packages wrap PostgreSQL queries for clusters, users, projects, apps, kubeconfig bindings, and events. Pagination, cursoring, and filters ensure API responses stay bounded.
- `internal/logging` writes request/audit logs and per-project append-only files under `logs/projects/<id>/`. Tail handlers stream data without loading entire files into memory.

### Scheduler and readiness

- `internal/service/healthscheduler.go` runs periodic health ticks against registered clusters, capturing status summaries exposed via `/v1/clusters/health` and `/v1/clusters/{id}/health`.

### Watcher pipeline

- `internal/watch/manager.go` builds dynamic informers for allowed kinds, persists resource versions to `internal/state`, and enforces label selectors + required keys to filter tenant workloads.
- `internal/sink/sink.go` batches events (max 200, 1s window), gzips payloads above 8 KiB, deduplicates via UID/resourceVersion, and retries with exponential backoff (250ms to 30s).
- The watcher binary (`cmd/kubeop-watcher/main.go`) exposes `/healthz`, `/readyz`, and `/metrics` on `:8081`, runs the sink loop, and emits optional heartbeat events when configured.

### Auto-deployment

- During cluster registration, `internal/service/service.go` evaluates `WatcherAutoDeploy` from configuration. When enabled it builds a `watcherdeploy.Config` (namespace, RBAC, PVC, image, token) and waits for readiness before completing the API response.

kubeOP keeps all automation within explicit services so operators can audit, extend, or disable components without redeploying controllers inside target clusters.
