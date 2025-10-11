Flavors

Built-in flavors provide quick defaults for CPU, Memory, replicas, and optional PVC size.

- f1-small: CPU 200m, Memory 256Mi, Replicas 1
- f2-medium: CPU 500m, Memory 512Mi, Replicas 2
- f3-large: CPU 1, Memory 1Gi, Replicas 2, PVC 5Gi

Usage

- In app deploy requests, set `"flavor": "f2-medium"` and optionally override via `resources`:
  - `{"name":"api","image":"org/api:latest","flavor":"f2-medium","resources":{"limits.cpu":"2"}}`

Custom resources

- You can omit `flavor` and specify `resources` directly:
  - `{"resources":{"requests.cpu":"300m","requests.memory":"300Mi","limits.cpu":"1","limits.memory":"1Gi"}}`
- The server validates against quotas and load balancer caps.

