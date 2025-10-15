# Architecture decisions

Short-form summaries of the key decisions behind kubeOP.

- **Out-of-cluster control plane** – All orchestration occurs outside managed clusters (`cmd/api`). Avoids cluster-level RBAC escalation and simplifies upgrades. Watchers provide cluster visibility when required.
- **Namespace-scoped tenancy** – Each user/project operates within a namespace seeded with ResourceQuotas, LimitRanges, and NetworkPolicies (`internal/service`). Provides deterministic isolation without per-cluster operators.
- **Encrypted kubeconfigs** – Stored kubeconfigs are encrypted using AES-GCM with a key derived from `KCFG_ENCRYPTION_KEY` (`internal/crypto`). Guarantees confidentiality even if the database is compromised.
- **JWT-based admin access** – Admin endpoints expect HMAC JWTs signed with `ADMIN_JWT_SECRET` (`internal/api/auth.go`). Keeps dependencies minimal and allows offline token generation.
- **Server-side apply** – Kubernetes resources are managed using server-side apply (`crclient.Apply`). kubeOP owns field managers and can safely reapply manifests without losing external changes.
- **Optional watcher deployment** – Auto-deployment of `kubeop-watcher` is controlled by configuration (`watcherdeploy`). Operators can disable it for air-gapped environments and run the watcher manually.
- **PostgreSQL persistence** – All metadata, events (when enabled), and kubeconfig records live in PostgreSQL (`internal/store`). Ensures transactional integrity for multi-tenant operations.
- **VitePress documentation** – Documentation lives in `docs/` and is built with VitePress to provide structured navigation, search, and diagrams.
