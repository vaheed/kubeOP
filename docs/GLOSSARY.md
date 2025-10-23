# Glossary

| Term | Definition |
| --- | --- |
| **App CRD** | Custom resource reconciled by `kubeop-operator` that represents a single application deployment. |
| **Cluster** | A registered Kubernetes cluster with encrypted kubeconfig and metadata (owner, environment, region, tags). |
| **Project** | Tenant-scoped workspace where applications, configs, and secrets live. Maps to a Kubernetes namespace. |
| **Tenant** | Logical owner (team, customer) that can hold multiple projects. Created via `/v1/users/bootstrap`. |
| **Delivery metadata** | Rendered manifests, SBOM fingerprints, and digests persisted for each deployment. |
| **Maintenance mode** | Toggle that blocks mutating APIs (projects, apps, clusters) with HTTP 503 responses. |
| **Quota** | ResourceQuota applied to tenant namespaces controlling CPU, memory, storage, and object counts. |
| **LimitRange** | Per-container defaults and bounds (CPU, memory, ephemeral storage) applied to tenant namespaces. |
| **Scheduler** | Background job that checks registered clusters and records health summaries. |
| **Release** | Immutable record of an app deployment, including source type, digests, and timestamps. |
| **AUTH_HEADER** | Shell array used in docs/scripts to store the `Authorization: Bearer` header for curl. |
| **Events bridge** | Optional `/v1/events/ingest` endpoint for forwarding Kubernetes events into kubeOP. |
| **Operator leader election** | Controller-runtime feature enabling multiple operator replicas to coordinate safely. |
| **PaaS domain** | Base domain used when generating ingress hosts for projects (`PAAS_DOMAIN`). |
