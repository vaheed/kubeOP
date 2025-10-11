Namespace-Scoped Kubeconfigs

Overview

- In v0.1.1 (default per-project mode), KubeOP creates a ServiceAccount in each project namespace and mints a token via the TokenRequest API.
- It builds a kubeconfig using the target cluster's server and CA, setting the current-context and namespace to the project's namespace.
- Labels reflect your data: the cluster/context name equals the registered cluster name; the kubeconfig user label is a stable identifier while authentication uses the ServiceAccount token.
- The kubeconfig is returned base64-encoded in API responses and stored encrypted in the database.

Token TTL

- Controlled by `SA_TOKEN_TTL_SECONDS` (default 3600). Renew by requesting again (future API).

Talos Notes

- Works with Talos clusters; no cloud-specific dependencies.
- Ensure cluster DNS is labeled appropriately (see ISOLATION.md).
