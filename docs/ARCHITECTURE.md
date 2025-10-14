Architecture

High-Level

- Out-of-cluster Go service exposing a REST API on port 8080.
- PostgreSQL stores users, clusters, projects, apps, kubeconfig bindings, and project events. Kubeconfigs and metadata are encrypted at rest where noted.
- Multi-cluster: controller-runtime client per cluster, constructed from stored kubeconfigs on demand. A simple in-memory cache avoids rebuilding clients repeatedly.
  Project provisioning: by default (v0.1.2), projects live in a user namespace (shared mode). You can switch to per-project namespaces by setting `PROJECTS_IN_USER_NAMESPACE=false`.

Packages

- `cmd/api`: main entrypoint; wires config, logging, store, service, and HTTP router.
- `cmd/kubeop-watcher`: informer-driven bridge streaming labelled
  resource changes into kubeOP’s ingest endpoint.
- `internal/config`: loads env and optional YAML config file (via `CONFIG_FILE`).
- `internal/logging`: builds zap-based JSON loggers with stdout + rotating file sinks.
- `internal/crypto`: AES-GCM utilities and key derivation from env.
- `internal/store`: database connection and embedded SQL migrations; CRUD for users/clusters/projects/apps/events.
- `internal/service/events.go`: normalises and records project events, redacting sensitive metadata before API responses.
- `internal/service`: business logic (encrypting kubeconfigs, validation) and DB orchestration.
- `internal/watcherdeploy`: renders Kubernetes manifests and readiness checks
  for the optional auto-deployed watcher bridge.
- `internal/service/healthscheduler.go`: reusable cluster health scheduler helper with bounded tick timeouts.
- `internal/service/manifests.go`: shared builders for NetworkPolicies and namespace RBAC to avoid drift.
- `internal/api`: HTTP router (chi), endpoints, auth middleware, health checks.
- `internal/kube`: multi-cluster client manager using controller-runtime + client-go.
- `internal/version`: build-time versioning variables.

Out-of-Cluster Design

- Runs as a container or standalone binary; no in-cluster permissions needed.
- Kubeconfigs for managed clusters are uploaded and stored encrypted; controller-runtime clients are initialized from decrypted kubeconfigs only when needed.
- The watcher bridge reuses kubeconfigs issued during cluster
  registration, persisting resource versions locally and delivering
  deduplicated batches to kubeOP over HTTPS with retry/backoff.
  When `WATCHER_AUTO_DEPLOY=true`, the API provisions the watcher deployment,
  RBAC, and supporting Secret/volume on registration and waits for readiness
  before returning.

Client Cache

- The `kube.Manager` caches `controller-runtime` clients keyed by cluster ID. If not present, it loads and decrypts the kubeconfig and constructs a new client.
- Cache invalidation is simple and in-memory for now; future phases can add TTLs, eviction, and metrics.

Extensibility

- Service layer is the pivot for adding tenants/projects/apps and future controllers.
- Endpoint and request types are versioned under `/v1` for now.

Diagram

```mermaid
flowchart LR
  subgraph "Client"
    U["Admin/Operator"]
    CLI["CLI / curl"]
  end

  subgraph "KubeOP API (Go)"
    R["Router / chi"]
    A["Auth Middleware"]
    SVC["Service Layer"]
    SCH["ClusterHealthScheduler"]
    LOG["Logging"]
  end

  subgraph "PostgreSQL"
    T1[("users\n+ deleted_at")]
    T2[("clusters\n+ enc kubeconfig")]
    T3[("projects\n+ quotas + enc kubeconfig")]
    T4[("apps\n+ deleted_at")]
    T5[("kubeconfigs\n+ secret metadata")]
    T6[("project_events\n+ meta jsonb")]
  end

  subgraph "Kubernetes"
    K8s1[("Cluster A")]
    K8s2[("Cluster B")]
  end

  U -- "Requests" --> CLI
  CLI --> R
  R --> A --> SVC
  SVC -- "CRUD soft delete" --> T1
  SVC -- "CRUD + enc kubeconfig" --> T2
  SVC -- "CRUD + quotas" --> T3
  SVC -- "CRUD soft delete" --> T4
  SVC -- "CRUD bindings" --> T5
  SVC -- "append events" --> T6
  SVC -- "decrypt + build client" --> K8s1
  SVC -- "decrypt + build client" --> K8s2
  SCH -- "tick summary" --> LOG
  SCH -- "list clusters" --> T2
  SCH -- "CheckCluster" --> SVC
  LOG --- R
  LOG --- A
  LOG --- SVC
  LOG --- SCH
```

User Flow

```mermaid
sequenceDiagram
  autonumber
  participant Admin as Admin/Operator
  participant API as KubeOP API
  participant DB as Postgres
  participant K8s as Kubernetes Cluster

  Note over Admin,API: Prereq: Register cluster (POST /v1/clusters)

  Admin->>API: POST /v1/users/bootstrap {userId|name+email,clusterId}
  API-->>Admin: 201 {namespace,kubeconfig_b64}

  Admin->>API: POST /v1/projects {userId,clusterId,name}
  API->>DB: insert project (namespace determined)
  API->>K8s: create Namespace + PSA label (per‑project mode)
  API->>K8s: apply ResourceQuota + LimitRange
  API->>K8s: create ServiceAccount + Role + RoleBinding
  API-->>Admin: 201 {project,kubeconfig_b64}

  Admin->>API: POST /v1/kubeconfigs {userId,clusterId,projectId?}
  API->>DB: upsert kubeconfig binding (user/project scope)
  API->>K8s: create ServiceAccount token Secret (wait for token/ca)
  API-->>Admin: 200 {id,namespace,secret_name,kubeconfig_b64}

  Note over Admin,API: Shared user namespace mode is optional via PROJECTS_IN_USER_NAMESPACE=true (pre‑provision namespace via external process).
```
Apps Flow

```mermaid
sequenceDiagram
  autonumber
  participant Admin as Admin/Operator
  participant API as KubeOP API
  participant DB as Postgres
  participant K8s as Kubernetes Cluster

  Admin->>API: POST /v1/projects/{id}/apps {image|manifests|helm}
  API->>DB: insert apps row
  API->>K8s: create Deployment/Service/Ingress (image) or apply labeled manifests
  API-->>Admin: 201 { appId }

  Admin->>API: DELETE /v1/projects/{id}/apps/{appId}
  API->>K8s: delete labeled resources in namespace
  API->>DB: set apps.deleted_at = now()
  API-->>Admin: 200 {status: deleted}
```

Background Scheduler

- `ClusterHealthScheduler` pulls cluster IDs from the store, runs `Service.CheckCluster` with per-tick timeouts, and logs a summary per execution.
- Future enhancements (see roadmap) will export Prometheus metrics from this helper.

Deletion

- All deletes use soft-delete in DB (set deleted_at) and remove Kubernetes resources where applicable.
