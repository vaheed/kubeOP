Quickstart: Apps

Goal

- Go from an empty installation to a running application (image, Helm, or Git) using numbered steps you can copy/paste.

Prereqs

1. Admin token with claim { "role": "admin" } → export AUTH_H="-H 'Authorization: Bearer $TOKEN'"
2. API online with database connected (`docker compose up -d --build` or `go run ./cmd/api`).
3. `jq` and `kubectl` installed for convenience.

Step 1 — Register a cluster (base64 kubeconfig)

```bash
B64=$(base64 -w0 < kubeconfig)                     # macOS/Linux
# Windows PowerShell: $B64 = [Convert]::ToBase64String([IO.File]::ReadAllBytes('kubeconfig'))
CLUSTER=$(curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d "$(jq -n --arg n 'my-cluster' --arg b64 "$B64" '{name:$n,kubeconfig_b64:$b64}')" \
  http://localhost:8080/v1/clusters)
CLUSTER_ID=$(echo "$CLUSTER" | jq -r '.id')
```

Step 2 — Bootstrap a user namespace (shared mode default)

```bash
USER_RESP=$(curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"alice@example.com","clusterId":"'"$CLUSTER_ID"'"}' \
  http://localhost:8080/v1/users/bootstrap)
USER_ID=$(echo "$USER_RESP" | jq -r '.user.id')
NS=$(echo "$USER_RESP" | jq -r '.namespace')
echo "$USER_RESP" | jq -r '.kubeconfig_b64' | base64 -d > user.kubeconfig
```
Step 3 — Create a project

```bash
PROJECT_RESP=$(curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"userId":"'"$USER_ID"'","clusterId":"'"$CLUSTER_ID"'","name":"demo"}' \
  http://localhost:8080/v1/projects)
PROJECT_ID=$(echo "$PROJECT_RESP" | jq -r '.project.id')
```

Step 4A — Deploy from a Docker image

```bash
APP_RESP=$(curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"name":"web","image":"nginx:1.27","ports":[{"containerPort":80,"servicePort":80,"serviceType":"LoadBalancer"}]}' \
  http://localhost:8080/v1/projects/'"$PROJECT_ID"'/apps)
APP_ID=$(echo "$APP_RESP" | jq -r '.appId // .app.id // .id')
```

> **Security note**: The default Pod Security level is `baseline`, which lets upstream images run without extra settings. If you
> set `POD_SECURITY_LEVEL=restricted`, swap to an unprivileged image (for example `nginxinc/nginx-unprivileged`) and listen on a
> high container port.

- Logs: `curl -s $AUTH_H http://localhost:8080/v1/projects/$PROJECT_ID/apps/$APP_ID/logs?tailLines=200`
- Access: `http://web.$NS.$PAAS_DOMAIN` (if wildcard ingress is enabled) or run `KUBECONFIG=./user.kubeconfig kubectl -n $NS get svc web -o wide`.

Step 4B — Deploy via Helm (Grafana example)

```bash
curl -s $AUTH_H -H 'Content-Type: application/json' \
  -d '{"name":"grafana","helm":{"chart":"https://grafana.github.io/helm-charts/grafana-8.5.13.tgz","values":{"adminUser":"admin","adminPassword":"StrongPassw0rd!"}}}' \
  http://localhost:8080/v1/projects/'"$PROJECT_ID"'/apps
```

> **Security guardrail**: Helm chart URLs must be `http(s)` and resolve to globally routable hosts. Requests that point to localhost, loopback, link-local, RFC1918, or any other non-public address space (even via DNS) are rejected server-side.

Step 4C — Deploy from Git (image + webhook trigger)

1. Deploy the app with repo + HMAC secret:
   ```bash
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"name":"api","image":"org/api:latest","repo":"org/repo","webhookSecret":"<hmac-secret>","ports":[{"containerPort":8080,"servicePort":80}]}' \
     http://localhost:8080/v1/projects/'"$PROJECT_ID"'/apps
   ```
2. Create a push webhook in your Git provider pointing to `http://<kubeop>/v1/webhooks/git`.
3. Send payloads with header `X-Hub-Signature-256: sha256=<hex(hmac(body, secret))>` so pushes trigger rollouts.
Step 5 — Attach ConfigMaps/Secrets (optional but recommended)

1. Create a ConfigMap or Secret in `$NS` via kubectl or API.
2. Attach all keys:
   ```bash
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"name":"app-config"}' \
     http://localhost:8080/v1/projects/'"$PROJECT_ID"'/apps/'"$APP_ID"'/configs/attach
   ```
3. Attach a subset with prefix:
   ```bash
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"name":"app-config","keys":["LOG_LEVEL"],"prefix":"APP_"}' \
     http://localhost:8080/v1/projects/'"$PROJECT_ID"'/apps/'"$APP_ID"'/configs/attach
   ```
4. Secrets follow the same pattern at `/secrets/attach`.
5. Detach to clean env vars when rotating resources:
   ```bash
   curl -s $AUTH_H -H 'Content-Type: application/json' \
     -d '{"name":"app-config"}' \
     http://localhost:8080/v1/projects/'"$PROJECT_ID"'/apps/'"$APP_ID"'/configs/detach
   ```

Environment knobs (ingress/LB/DNS)

- `PAAS_DOMAIN`, `PAAS_WILDCARD_ENABLED=true` → serve `{app}.{namespace}.{PAAS_DOMAIN}`.
- `LB_DRIVER=metallb` with optional `LB_METALLB_POOL`.
- `MAX_LOADBALANCERS_PER_PROJECT` (default 1) or per-project override `services.loadbalancers`.
- Optional DNS automation: set `EXTERNAL_DNS_PROVIDER` (`cloudflare` or `powerdns`) plus provider secrets to upsert hosts.

Next steps

- Explore docs/APPS.md for advanced deployment flows (manifests, flavors, scaling, rollouts).
- Read docs/CI_WEBHOOKS.md to wire Git providers.
- Review docs/INGRESS_LB.md for ingress/LB tuning and DNS cleanup expectations.
