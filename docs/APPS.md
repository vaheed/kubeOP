# Application workflows

kubeOP manages application lifecycles from a single API. This page highlights the most common flows and the new validation workflow.

## Dry-run before deploy

1. Collect the project ID from `/v1/projects` or `/v1/users/bootstrap`.
2. POST the app spec to `/v1/apps/validate`.
3. Inspect the JSON output for:
   - `kubeName` – deterministic resource name that will be used for Deployment/Service/Ingress.
   - `replicas` and `resources` – effective values after applying flavors or overrides.
   - `loadBalancers` – ensures the request stays within quota limits.
   - `renderedObjects` – summary of Kubernetes objects produced by raw manifests or Helm charts.
4. Fix any issues (missing names, quota errors, Helm rendering failures) before calling `/v1/projects/{id}/apps`.

## Deploy after validation

Send the same payload to `/v1/projects/{id}/apps` to create the resources. kubeOP persists the deployment metadata, applies labels, and emits audit events so you can track history via `/v1/projects/{id}/events`.

## Troubleshooting tips

- Validation errors return HTTP `400` with an `error` message. Common cases include unknown flavors, exceeding load balancer quotas, or malformed YAML/Helm manifests.
- Warnings appear in the `warnings[]` array and do not block deployment, but they call out missing names or namespaces that kubeOP will auto-fill.
- When validation succeeds but deployment later fails, check the project event feed or the per-app log file under `${LOGS_ROOT}/projects/<project_id>/apps/<app_id>/deploy.log`.
