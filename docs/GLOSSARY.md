# Glossary

| Term | Definition |
| --- | --- |
| **App** | Custom resource representing an application deployment managed by kubeOP. |
| **Cluster** | A registered Kubernetes cluster. kubeOP stores kubeconfig, metadata, and health history. |
| **Credential store** | Encrypted storage for Git or container registry credentials scoped to a user or project. |
| **`kubeop-operator`** | Controller-runtime manager installed into each cluster to reconcile `App` CRDs. |
| **Maintenance mode** | API state that blocks mutating operations during upgrades. Enabled via `/v1/admin/maintenance`. |
| **Project** | Application workspace within a tenant namespace, with quotas and access controls. |
| **Tenant** | Logical owner of projects and namespaces, created via `/v1/users/bootstrap`. |
| **SBOM** | Software Bill of Materials generated during app validation and stored with releases. |
| **LOGS_ROOT** | Filesystem directory storing per-project logs and event archives. |
