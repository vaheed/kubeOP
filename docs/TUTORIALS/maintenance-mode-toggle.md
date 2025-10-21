# Pause kubeOP during control plane maintenance

This tutorial walks through enabling and disabling kubeOP's global maintenance mode
so operators can pause mutating API flows (cluster registration, project/app changes,
template deployments, quota updates, etc.) while the control plane is upgraded.

## Prerequisites

- kubeOP API running locally (Docker Compose or `go run ./cmd/api`).
- Admin authentication header exported as `AUTH_H` (see README Quickstart step 4).
- A project/app you can use to demonstrate blocked operations.

## 1. Inspect the current maintenance state

```bash
curl -s $AUTH_H http://localhost:8080/v1/admin/maintenance | jq
```

The response shows whether maintenance mode is enabled along with the last actor and
update timestamp:

```json
{
  "enabled": false,
  "message": "",
  "updatedAt": "2025-10-31T12:00:00Z",
  "updatedBy": "system"
}
```

## 2. Enable maintenance mode with a descriptive message

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"enabled":true,"message":"Platform upgrade in progress"}' \
  http://localhost:8080/v1/admin/maintenance | jq
```

All API replicas persist the new state in PostgreSQL immediately, so subsequent calls
return the same payload.

## 3. Attempt a mutating API call (blocked with HTTP 503)

With maintenance enabled, mutating endpoints (app deploy/scale/image update, project
create/delete, cluster metadata updates, etc.) short-circuit. For example, deploying an
app now fails fast:

```bash
curl -s -o /tmp/resp.json -w '%{http_code}\n' \
  $AUTH_H -H 'Content-Type: application/json' \
  -d '{"name":"web","image":"ghcr.io/library/nginx:1.27"}' \
  http://localhost:8080/v1/projects/<project-id>/apps
cat /tmp/resp.json | jq
```

The status code is `503` and the body echoes the operator message:

```json
{"error":"maintenance mode enabled: Platform upgrade in progress"}
```

## 4. Disable maintenance mode when the upgrade completes

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"enabled":false}' \
  http://localhost:8080/v1/admin/maintenance | jq
```

Mutating API calls now succeed again, allowing queued deployments or pipeline retries to
continue automatically.

## Troubleshooting

- The maintenance toggle is stored in the `maintenance_state` table. If the API returns a
  database error while toggling, ensure migrations have been applied (`go run ./cmd/api`
  runs them automatically) and that the API can reach PostgreSQL.
- Maintenance mode applies only to mutating admin APIs; read-only endpoints (`GET`,
  `/metrics`, `/healthz`, etc.) remain available for observability throughout the upgrade.
