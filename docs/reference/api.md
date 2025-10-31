---
outline: deep
---

# API: End‑to‑End Walkthrough

This page shows a complete flow using only the Manager API. It covers: registering a cluster, bootstrapping the platform, creating a tenant/project/app, enforcing policy, issuing kubeconfigs, ingesting usage, and exporting an invoice.

Assumptions

- Manager is running at `http://localhost:18080` (Docker Compose)
- Postgres is healthy; KMS key configured (or dev mode enabled)
- Auth is disabled for brevity (`KUBEOP_REQUIRE_AUTH=false`). If auth is enabled, add `Authorization: Bearer <admin-token>` to every request.

Set base URL:

```bash
export MGR=http://localhost:18080
```

## 1) Register a Cluster (encrypted kubeconfig)

```bash
export CLUSTER_NAME=kind-kubeop
# Cross‑platform base64 (Linux/macOS)
export KCFG_B64=$(base64 -w0 < ~/.kube/config 2>/dev/null || base64 < ~/.kube/config | tr -d '\n')

curl -sS -X POST "$MGR/v1/clusters" \
  -H 'Content-Type: application/json' \
  -d '{"name":"'"$CLUSTER_NAME"'","kubeconfig":"'"$KCFG_B64"'","autoBootstrap":true,"installAdmission":true,"withMocks":true}' | tee /tmp/cluster.json

export CLUSTER_ID=$(jq -r .id /tmp/cluster.json)
```

The kubeconfig is stored encrypted at rest via KMS. `autoBootstrap:true` applies CRDs and the operator Deployment immediately.

Verify platform state on that cluster:

```bash
curl -sS "$MGR/v1/clusters/$CLUSTER_ID/status"
# => {"operator":true,"admission":<may be true if installed via Helm>,"webhookCABundle":<bool>}

Poll the cluster ready endpoint (200 OK when operator and admission are Ready and CABundle set):

```bash
until curl -sf -o /dev/null "$MGR/v1/clusters/$CLUSTER_ID/ready"; do sleep 3; done
echo "Cluster ready"
```
```

## 2) Create a Tenant bound to the Cluster

```bash
curl -sS -X POST "$MGR/v1/tenants" \
  -H 'Content-Type: application/json' \
  -d '{"name":"acme","clusterID":"'"$CLUSTER_ID"'"}' | tee /tmp/tenant.json

export TENANT_ID=$(jq -r .id /tmp/tenant.json)
```

## 3) Create a Project in that Tenant

```bash
curl -sS -X POST "$MGR/v1/projects" \
  -H 'Content-Type: application/json' \
  -d '{"tenantID":"'"$TENANT_ID"'","name":"web"}' | tee /tmp/project.json

export PROJECT_ID=$(jq -r .id /tmp/project.json)
```

## 4) Set Policy Guardrails (allowlist + egress + quota)

```bash
curl -sS -X PUT "$MGR/v1/platform/policy" -H 'Content-Type: application/json' -d '{
  "imageAllowlist": ["docker.io","ghcr.io"],
  "egressBaseline": ["10.0.0.0/8","172.16.0.0/12"],
  "quotaMax": {"requestsCPU":"4","requestsMemory":"8Gi"}
}' -i
```

The Manager syncs policy into a ConfigMap and rolls the admission Deployment to pick up changes (if admission is installed on the cluster).

## 5) Create an App (DB record)

```bash
curl -sS -X POST "$MGR/v1/apps" \
  -H 'Content-Type: application/json' \
  -d '{"projectID":"'"$PROJECT_ID"'","name":"web","image":"docker.io/library/nginx:1.25","host":"web.local"}' | tee /tmp/app.json

export APP_ID=$(jq -r .id /tmp/app.json)
```

If you install the operator+admission via the Helm chart, create the Kubernetes `App` CR for rollout. Admission enforces the image allowlist and network policy baselines.

## 6) Issue a project‑scoped token and kubeconfig

```bash
curl -sS -X POST "$MGR/v1/jwt/project" -H 'Content-Type: application/json' \
  -d '{"ProjectID":"'"$PROJECT_ID"'","TTLMinutes":60}' | tee /tmp/token.json

curl -sS "$MGR/v1/kubeconfigs/project/$PROJECT_ID" | jq -r .kubeconfig > kubeconfig.yaml
```

## 7) Usage ingestion → Invoice export

```bash
HOUR=$(date -u -d '1 hour ago' +%Y-%m-%dT%H:00:00Z 2>/dev/null || date -u -v -1H +%Y-%m-%dT%H:00:00Z)
curl -sS -X POST "$MGR/v1/usage/ingest" -H 'Content-Type: application/json' \
  -d '[{"ts":"'$HOUR'","tenant_id":"'$TENANT_ID'","cpu_milli":500,"mem_mib":1024}]'

curl -sS "$MGR/v1/invoices/$TENANT_ID" | jq
```

## 8) Optional cluster ops via API

- Bootstrap again (e.g., after upgrades):

```bash
curl -sS -X POST "$MGR/v1/platform/bootstrap?clusterID=$CLUSTER_ID" -H 'Content-Type: application/json' \
  -d '{"InstallMetricsServer":false,"Mocks":false}'
```

- Operator autoscale (HPA) on that cluster:

```bash
curl -sS -X POST "$MGR/v1/platform/autoscale?clusterID=$CLUSTER_ID" -H 'Content-Type: application/json' \
  -d '{"Enabled":true,"Min":1,"Max":3,"TargetCPU":70}' -i
```

## Reference

- OpenAPI: `GET /openapi.json`
- Health: `/healthz`; Ready: `/readyz`; Version: `/version`; Metrics: `/metrics`
- Clusters: `POST/GET /v1/clusters`, `GET/DELETE /v1/clusters/{id}`
- Tenants: `POST/GET/PUT/PATCH /v1/tenants`, `GET/DELETE /v1/tenants/{id}`
- Projects: `POST/GET/PUT/PATCH /v1/projects`, `GET/DELETE /v1/projects/{id}`
- Apps: `POST/GET/PUT/PATCH /v1/apps`, `GET/DELETE /v1/apps/{id}`
- Usage: `POST /v1/usage/ingest`, `GET /v1/usage/snapshot`
- Invoice: `GET /v1/invoices/{tenantID}`
- Tokens/Kubeconfigs: `POST /v1/jwt/project`, `GET /v1/kubeconfigs/project/{id}`
- Policy/Bootstrap/Status/Autoscale: `/v1/platform/*` (admin)

## Errors

- 400 – invalid input
- 401 – missing/invalid token (when auth enabled)
- 403 – forbidden by role/scope
- 404 – not found
- 5xx – server/database errors
