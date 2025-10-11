Namespace-Scoped Kubeconfigs

Overview

- On project creation, KubeOP creates a ServiceAccount in the project namespace and mints a token via the TokenRequest API.
- It builds a kubeconfig using the target cluster's server and CA and sets the namespace.
- The kubeconfig is returned base64-encoded and stored encrypted.

Token TTL

- Controlled by `SA_TOKEN_TTL_SECONDS` (default 3600). Renew by requesting again (future API).

Talos Notes

- Works with Talos clusters; no cloud-specific dependencies.
- Ensure cluster DNS is labeled appropriately (see ISOLATION.md).

