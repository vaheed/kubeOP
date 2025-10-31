---
outline: deep
---

# Manager API Reference

Base URL defaults to `http://localhost:18080` when running via Docker Compose. All responses are JSON. The OpenAPI document is available at `/openapi.json`.

Service endpoints (all binaries):

- `/healthz` – liveness probe
- `/readyz` – readiness (checks DB, KMS)
- `/version` – `{ service, version, gitCommit, buildDate }`
- `/metrics` – Prometheus metrics (go/process + domain)

Authentication (JWT HS256):

- Enable with `KUBEOP_REQUIRE_AUTH=true` and set `KUBEOP_JWT_SIGNING_KEY`.
- Claims carry a `role` and `scope`:
  - `admin` – full access
  - `tenant` – `scope="tenant:<TENANT_ID>"`
  - `project` – `scope="project:<PROJECT_ID>"`
- Include `Authorization: Bearer <token>` on requests when auth is enabled.
- Mint project‑scoped tokens via `POST /v1/jwt/project` (admin‑only).

Tip: For local demos, set `KUBEOP_REQUIRE_AUTH=false`.

Environment setup for snippets:

```bash
export MGR=http://localhost:18080
```

## Tenants

Create (admin):

```bash
curl -sS -X POST "$MGR/v1/tenants" \
  -H 'Content-Type: application/json' \
  -d '{"name":"acme"}'
# => {"id":"t_...","name":"acme","created_at":"..."}
```

List (admin):

```bash
curl -sS "$MGR/v1/tenants"
```

Get (admin or tenant‑role matching the path id):

```bash
TENANT_ID=...
curl -sS "$MGR/v1/tenants/$TENANT_ID"
```

Update (admin):

```bash
curl -sS -X PUT "$MGR/v1/tenants" -H 'Content-Type: application/json' \
  -d '{"ID":"'$TENANT_ID'","Name":"acme-inc"}' -i
```

Delete (admin):

```bash
curl -sS -X DELETE "$MGR/v1/tenants/$TENANT_ID" -i
```

## Projects

Create (admin or tenant‑role for the target tenant):

```bash
curl -sS -X POST "$MGR/v1/projects" -H 'Content-Type: application/json' \
  -d '{"tenantID":"'$TENANT_ID'","name":"web"}'
# => {"id":"p_...","tenant_id":"...","name":"web"}
PROJECT_ID=...
```

List (admin; tenant must filter by own tenant):

```bash
curl -sS "$MGR/v1/projects?tenantID=$TENANT_ID"
```

Get (admin or tenant owning it or project‑role for this project):

```bash
curl -sS "$MGR/v1/projects/$PROJECT_ID"
```

Update name (admin):

```bash
curl -sS -X PATCH "$MGR/v1/projects" -H 'Content-Type: application/json' \
  -d '{"ID":"'$PROJECT_ID'","Name":"web-main"}' -i
```

Delete (admin):

```bash
curl -sS -X DELETE "$MGR/v1/projects/$PROJECT_ID" -i
```

## Apps

Create (admin or project‑role):

```bash
curl -sS -X POST "$MGR/v1/apps" -H 'Content-Type: application/json' \
  -d '{"projectID":"'$PROJECT_ID'","name":"web","image":"docker.io/library/nginx:1.25","host":"web.local"}'
# => {"id":"a_...","project_id":"...","name":"web","image":"..."}
APP_ID=...
```

Registry allowlist: manager enforces `KUBEOP_IMAGE_ALLOWLIST` (or platform policy, see below). Attempts to use a non‑allowlisted registry return `400`.

List (admin; project‑role must filter by own project):

```bash
curl -sS "$MGR/v1/apps?projectID=$PROJECT_ID"
```

Get (admin or project‑role for this project):

```bash
curl -sS "$MGR/v1/apps/$APP_ID"
```

Update (admin):

```bash
curl -sS -X PUT "$MGR/v1/apps" -H 'Content-Type: application/json' \
  -d '{"ID":"'$APP_ID'","Name":"web","Image":"docker.io/library/nginx:1.26","Host":"web.local"}' -i
```

Delete (admin):

```bash
curl -sS -X DELETE "$MGR/v1/apps/$APP_ID" -i
```

## Usage & Invoices

Ingest hourly usage (admin or matching tenant when auth enabled):

```bash
HOUR=$(date -u -d '1 hour ago' +%Y-%m-%dT%H:00:00Z)
curl -sS -X POST "$MGR/v1/usage/ingest" -H 'Content-Type: application/json' \
  -d '[{"ts":"'$HOUR'","tenant_id":"'$TENANT_ID'","cpu_milli":100,"mem_mib":200}]'
```

Snapshot totals:

```bash
curl -sS "$MGR/v1/usage/snapshot"
# => {"totals":{"cpu_milli":..,"mem_mib":..}}
```

Invoice (current month):

```bash
curl -sS "$MGR/v1/invoices/$TENANT_ID"
# => {"tenant_id":"...","start":"...","end":"...","lines":[...],"subtotal":123.45}
```

Per‑tenant rates may be configured in the database (`tenant_rates` table). When unset, rates fall back to `KUBEOP_RATE_CPU_MILLI` and `KUBEOP_RATE_MEM_MIB` environment variables.

## Kubeconfigs and Tokens

Mint a project‑scoped JWT (admin):

```bash
curl -sS -X POST "$MGR/v1/jwt/project" -H 'Content-Type: application/json' \
  -d '{"ProjectID":"'$PROJECT_ID'","TTLMinutes":60}'
# => {"token":"<jwt>"}
```

Issue kubeconfig by project (admin or project‑role):

```bash
curl -sS "$MGR/v1/kubeconfigs/project/$PROJECT_ID" | jq -r .kubeconfig > kubeconfig.yaml
```

Issue kubeconfig for an arbitrary namespace (admin):

```bash
curl -sS "$MGR/v1/kubeconfigs/kubeop-acme-web" | jq -r .kubeconfig
```

Note: kubeconfigs are returned inline as YAML in a JSON field.

## Platform Management

Policy (admin):

```bash
# Get current policy
curl -sS "$MGR/v1/platform/policy"

# Update policy: image allowlist, egress baseline CIDRs, quota ceilings
curl -sS -X PUT "$MGR/v1/platform/policy" -H 'Content-Type: application/json' -d '{
  "imageAllowlist": ["docker.io","ghcr.io"],
  "egressBaseline": ["10.0.0.0/8","172.16.0.0/12"],
  "quotaMax": {"requestsCPU":"4","requestsMemory":"8Gi"}
}' -i
```

Autoscale HPA for operator (admin):

```bash
curl -sS -X POST "$MGR/v1/platform/autoscale" -H 'Content-Type: application/json' \
  -d '{"Enabled":true,"Min":1,"Max":3,"TargetCPU":70}' -i
```

Bootstrap (dev helper, admin): applies CRDs and operator manifests, optionally installs metrics‑server.

```bash
curl -sS -X POST "$MGR/v1/platform/bootstrap" -H 'Content-Type: application/json' \
  -d '{"InstallMetricsServer":true,"Mocks":true}'
```

Status (admin):

```bash
curl -sS "$MGR/v1/platform/status"
# => {"operator":true,"admission":true,"webhookCABundle":true}
```

## Errors

- 400 – invalid input
- 401 – missing/invalid token (when auth enabled)
- 403 – forbidden by role/scope
- 404 – not found
- 5xx – server/database errors

