Namespace-Scoped Kubeconfigs

Overview

- KubeOP ensures a ServiceAccount, Role, and RoleBinding per user/project namespace and creates a `kubernetes.io/service-account-token` Secret annotated for that ServiceAccount.
- It builds a kubeconfig using the target cluster's server and the Secret-provided `token`/`ca.crt`, setting the current-context and namespace to the scope.
- Labels reflect your data: the cluster/context name equals the registered cluster name; the kubeconfig user label is a stable identifier while authentication uses the ServiceAccount token.
  - The kubeconfig "user" entry is a friendly label for local readability (e.g., the user's name or email). Kubernetes still authenticates as the ServiceAccount (identity like `system:serviceaccount:<ns>:user-sa`).
- The kubeconfig is returned base64-encoded in API responses, stored encrypted in the database, and the binding metadata (Secret and ServiceAccount) is recorded for later rotation/revocation.

Token Lifecycle

- Tokens are non-expiring until revoked. Use `POST /v1/kubeconfigs/rotate` to mint a new Secret-backed token and `DELETE /v1/kubeconfigs/{id}` to revoke a binding (deletes the Secret and, when exclusive, the ServiceAccount).

Talos Notes

- Works with Talos clusters; no cloud-specific dependencies.
- Ensure cluster DNS is labeled appropriately (see ISOLATION.md).

RBAC Scope and Verification

- User kubeconfigs are namespace-scoped by design to prevent access outside the user's namespace. They can manage the curated set
  of namespaced workloads and configuration resources (Pods, Services, ConfigMaps, Secrets, PersistentVolumeClaims, Events,
  Deployments/ReplicaSets/StatefulSets/DaemonSets, Jobs/CronJobs, and Ingresses) and can patch the `deployments/scale` and
  `statefulsets/scale` subresources to change replica counts.
- Cluster-scoped operations such as `kubectl get ns` or `kubectl get nodes` remain forbidden and will return "Forbidden".
- Verify access with namespaced commands, for example:
  - `kubectl -n user-<userId> get pods`
  - `kubectl -n user-<userId> get resourcequota`
  - `kubectl --kubeconfig kubeconfig.yaml auth can-i create deployments -n user-<userId>`
  - `kubectl --kubeconfig kubeconfig.yaml auth can-i get secrets -n user-<userId>`
  - `kubectl --kubeconfig kubeconfig.yaml -n user-<userId> scale deployment web-02 --replicas=2`
  - `kubectl --kubeconfig kubeconfig.yaml get namespaces` (should return "Forbidden")
