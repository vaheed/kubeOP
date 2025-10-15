# Quotas and limits

kubeOP enforces ResourceQuota and LimitRange objects for every tenant namespace. This guide explains defaults, inspection, and overrides.

## Default policy

- Namespace ResourceQuota values derive from `KUBEOP_DEFAULT_*` variables (see [Configuration](../configuration.md)).
- LimitRange values derive from `KUBEOP_DEFAULT_LR_*` settings.
- Objects are labelled with `managed-by=kubeop-operator` and re-applied whenever projects are created, suspended, unsuspended, or quota overrides change.
- Load balancer usage is capped globally by `MAX_LOADBALANCERS_PER_PROJECT` and per-namespace by `KUBEOP_DEFAULT_SERVICES_LOADBALANCERS`.

## Inspect project quota

`GET /v1/projects/{id}/quota` returns:

```json
{
  "defaults": {"limits.cpu": "4", "limits.memory": "8Gi", ...},
  "overrides": {"limits.cpu": "6"},
  "usage": {"limits.cpu": "2", "requests.memory": "3Gi"},
  "limitRange": {
    "max": {"cpu": "2", "memory": "2Gi"},
    "default": {"cpu": "500m", "memory": "512Mi"},
    "defaultRequest": {"cpu": "300m", "memory": "256Mi"}
  }
}
```

Usage values are fetched directly from the Kubernetes API. Overrides reflect values applied via previous `PATCH` operations.

## Override quota values

`PATCH /v1/projects/{id}/quota` accepts a payload with partial overrides. Provide keys as they appear in ResourceQuota (`limits.cpu`, `requests.memory`, etc.).

```bash
curl -s -X PATCH \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{
    "overrides": {
      "limits.cpu": "6",
      "requests.memory": "12Gi"
    }
  }' \
  http://localhost:8080/v1/projects/<project-id>/quota | jq
```

kubeOP validates overrides against namespace limits (must remain positive numbers) and applies them via server-side apply. Overrides persist in PostgreSQL so re-deploys keep the custom values.

## Suspend and resume behaviour

- `POST /v1/projects/{id}/suspend` removes Services/Ingresses and scales workloads to zero but leaves quotas in place.
- `POST /v1/projects/{id}/unsuspend` re-applies quotas and limit ranges before recreating workloads.
- Deleting a project removes override records; re-creating a project starts with defaults.

## Troubleshooting quota errors

- `max load balancers reached` – Service payload requested more LoadBalancer ports than allowed. Update `MAX_LOADBALANCERS_PER_PROJECT` or remove existing LoadBalancer Services.
- `quota exceeded` responses from Kubernetes appear when requested resources exceed available limits. Inspect `usage` from the quota endpoint and adjust overrides accordingly.
- If quota objects were modified manually, kubeOP reconciles them on the next suspend/resume or quota patch. To force reconciliation immediately, issue a no-op patch (`{ "overrides": {} }`).
