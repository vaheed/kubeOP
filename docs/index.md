# kubeOP

kubeOP is an out-of-cluster control plane that lets operators register Kubernetes clusters, enforce multi-tenant boundaries, and deliver applications through one REST API.

## kubeOP at a glance

- **Control plane first** – The API (Go `chi` router) mediates every request through authentication and audit middleware before delegating to the service layer and PostgreSQL-backed store. Health and readiness endpoints expose scheduler state and dependency checks for probes.
- **Tenant scaffolding** – Namespace bootstrapper utilities apply managed `ResourceQuota` and `LimitRange` objects, ensuring every tenant namespace is annotated and drift-corrected during suspend, resume, or quota updates.
- **Application delivery** – Operators deploy container images, Helm charts, or raw manifests. kubeOP renders Kubernetes objects, attaches secrets/configs, and surfaces rollout status by inspecting deployments, services, ingresses, and pods with controller-runtime clients.
- **Operational insight** – All mutating requests generate structured audit logs, disk-backed project/app logs, and metrics under `/metrics`. Log download endpoints and tail filters support incident response without shell access.

Head to the [5-minute quickstart](./getting-started.md) to launch kubeOP locally.

## Component snapshot

```mermaid
flowchart LR
    subgraph Clients
        CLI[curl / CI]
        UI[Internal tools]
    end
    subgraph ControlPlane
        API[internal/api<br/>chi router]
        SVC[internal/service]
        STORE[(PostgreSQL<br/>internal/store)]
        SCHED[Cluster health scheduler]
        FILES[logs/, events JSONL]
    end
        SINK[internal/sink batching]
    end
    CLI -->|JWT auth| API
    UI --> API
    API -->|request context| SVC
    SVC --> STORE
    SVC -->|logs & events| FILES
    SVC -->|kube clients| CL[Target clusters]
    SVC -->|auto deploy| WD
    WD -->|label-filtered events| SINK
    SINK -->|HTTPS batches| API
    SCHED -->|status & metrics| STORE
    SCHED --> API
    subgraph Clusters
        CL[(Namespaces, workloads)]
    end
```

## Next steps

- Learn the [architecture and data flows](./architecture.md).
- Follow the [operations runbook](./operations.md) for day-2 care.
- Try the [app validation walkthrough](./guides/app-validation.md) to dry-run deployments.
- Explore the [release history guide](./guides/app-release-history.md) to audit deployment digests and warnings.
- Manage clusters with the [inventory and health tutorial](./TUTORIALS/cluster-inventory-service.md).
- Explore the [API reference](./api/README.md) with runnable examples.
- Review the [security policy](./security.md) for reporting and hardening guidance.
