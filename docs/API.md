# API highlights

## Validate application specs before deployment

Use `POST /v1/apps/validate` to dry-run an application payload before deploying it to a project. The endpoint returns the computed Kubernetes resource names, replicas, resource overrides, load balancer quota consumption, and a summary of manifests that would be applied.

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{
        "projectId": "<project-id>",
        "name": "web",
        "image": "ghcr.io/example/web:1.2.3",
        "ports": [
          {"containerPort": 80, "servicePort": 80, "serviceType": "LoadBalancer"}
        ]
      }' \
  http://localhost:8080/v1/apps/validate | jq
```

A successful response resembles:

```json
{
  "projectId": "f8f2d7aa-2c86-4e7b-8b42-0c741ef7d4f2",
  "projectNamespace": "tenant-f8f2d7aa",
  "clusterId": "cluster-1",
  "source": "image",
  "kubeName": "web-abc123def",
  "replicas": 1,
  "loadBalancers": {"requested": 1, "existing": 0, "limit": 2},
  "renderedObjects": [
    {"kind": "Deployment", "name": "web-abc123def", "namespace": "tenant-f8f2d7aa"},
    {"kind": "Service", "name": "web-abc123def", "namespace": "tenant-f8f2d7aa"},
    {"kind": "Ingress", "name": "web-abc123def", "namespace": "tenant-f8f2d7aa"}
  ]
}
```

Validation errors return HTTP `400` with a descriptive `error` message.
