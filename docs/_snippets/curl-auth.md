```bash
set -euo pipefail
export KUBEOP_TOKEN="${KUBEOP_TOKEN:-<admin-jwt>}"
AUTH_HEADER=("-H" "Authorization: Bearer ${KUBEOP_TOKEN}")
```
