# Zero to Production with kubeOP

## 1. Prerequisites and configuration

kubeOP runs as an out-of-cluster control plane. Before you begin, gather:

- A Linux/macOS workstation (or bastion) with `curl`, `jq`, `kubectl`, `base64`, `openssl`, `docker`, and `docker compose`.
- An accessible Kubernetes cluster with cert-manager installed and an admin kubeconfig saved locally.
- Control of a domain that can host `api.<domain>` plus wildcard application records.
- Cloudflare API credentials (zone ID + token) for DNS automation.
- An admin JWT secret; kubeOP validates admin tokens signed with this secret and containing `{"role":"admin"}`.

Environment variables drive every kubeOP feature. The following table highlights the values referenced throughout the guide:

| Variable | Default | Purpose & notes |
| --- | --- | --- |
| `KUBEOP_BASE_URL` | _required_ | External HTTPS origin for the API. Watchers use it for handshakes and event ingest. |
| `ALLOW_INSECURE_HTTP` | `false` | Permits `http://` watcher traffic. Keep `false` in production. |
| `DATABASE_URL` | `postgres://postgres:postgres@postgres:5432/kubeop?sslmode=disable` | PostgreSQL DSN for the API and background jobs. |
| `ADMIN_JWT_SECRET` | _required_ | HMAC key for admin JWTs. Use a random 32-byte value. |
| `KCFG_ENCRYPTION_KEY` | _required_ | AES-GCM key protecting kubeconfigs at rest. Never reuse across environments. |
| `LOGS_ROOT`, `LOG_DIR` | `/var/log/kubeop` | Directory tree for API, project, and app logs. Ensure it is writable. |
| `PAAS_DOMAIN` | _required_ | Shared domain used to mint `<app-full>.<project>.<cluster>.<PAAS_DOMAIN>` hostnames. `<app-full>` joins the slugified app name with a deterministic short hash (for example, `web-02-f7f88c5b4-4ldbq`). |
| `PAAS_WILDCARD_ENABLED` | `false` | Enable to auto-generate application FQDNs. |
| `ENABLE_CERT_MANAGER` | `false` | Enables cert-manager Certificate resources for issued TLS. |
| `DNS_PROVIDER` | _empty_ | Set to `cloudflare`, `http`, or `powerdns` to turn on DNS automation. |
| `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ZONE_ID` | _empty_ | Required when `DNS_PROVIDER=cloudflare`. |
| `WATCHER_AUTO_DEPLOY` | auto | Automatically flips on when `KUBEOP_BASE_URL` is set. Controls watcher rollout. |
| `WATCHER_NAMESPACE`, `WATCHER_DEPLOYMENT_NAME`, `WATCH_NAMESPACE_PREFIXES` | see `.env.example` | Default watcher deployment (`kubeop-watcher`) and namespace filters (`user-`). |
| `K8S_EVENTS_BRIDGE` | `false` | Accepts watcher event batches at `/v1/events/ingest` when `true`. |

Populate the critical values and derive secure defaults with the following block. The prompts keep credentials out of history files.

```bash
set -euo pipefail

read -rp "Enter the kubeOP apps domain (e.g. apps.example.com): " PAAS_DOMAIN
read -rp "Enter the Cloudflare Zone ID for ${PAAS_DOMAIN}: " CLOUDFLARE_ZONE_ID
read -rsp "Enter the Cloudflare API token: " CLOUDFLARE_API_TOKEN; echo
read -rp "Path to the admin kubeconfig for your target cluster: " TARGET_KUBECONFIG

export PAAS_DOMAIN CLOUDFLARE_ZONE_ID CLOUDFLARE_API_TOKEN TARGET_KUBECONFIG
export KUBEOP_BASE_URL="https://api.${PAAS_DOMAIN}"
export API_ORIGIN="$KUBEOP_BASE_URL"
export ALLOW_INSECURE_HTTP=false
export LOGS_ROOT="$HOME/kubeop-logs"
export LOG_DIR="$LOGS_ROOT"
export DNS_PROVIDER=cloudflare
export ENABLE_CERT_MANAGER=true
export PAAS_WILDCARD_ENABLED=true
export WATCH_NAMESPACE_PREFIXES="user-"
export WATCHER_NAMESPACE="kubeop-system"
export WATCHER_DEPLOYMENT_NAME="kubeop-watcher"
export K8S_EVENTS_BRIDGE=true
export PROJECTS_IN_USER_NAMESPACE=false
mkdir -p "$LOGS_ROOT"

export ADMIN_JWT_SECRET="$(openssl rand -hex 32)"
export KCFG_ENCRYPTION_KEY="$(openssl rand -hex 32)"
export TOKEN="$(python - <<'PY'
import base64, hashlib, hmac, json, os, time

def b64url(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).decode().rstrip('=')

secret = bytes.fromhex(os.environ['ADMIN_JWT_SECRET'])
header = {"alg": "HS256", "typ": "JWT"}
payload = {
    "role": "admin",
    "sub": "platform-admin",
    "iat": int(time.time()),
    "exp": int(time.time()) + 12 * 3600,
}
segments = [b64url(json.dumps(header, separators=(',', ':')).encode()),
            b64url(json.dumps(payload, separators=(',', ':')).encode())]
message = '.'.join(segments).encode()
signature = b64url(hmac.new(secret, message, hashlib.sha256).digest())
print('.'.join(segments + [signature]), end='')
PY)"
```

> ℹ️ The generated admin token expires after 12 hours. Re-run the Python snippet to mint a new JWT when required.

## 2. Clone kubeOP and expose the API

Start the API and PostgreSQL with Docker Compose, append your overrides to `.env`, and front the API with Caddy for TLS on `api.${PAAS_DOMAIN}`.

```bash
set -euo pipefail

git clone https://github.com/vaheed/kubeOP.git
cd kubeOP
cp .env.example .env
cat <<'ENV' >> .env
KUBEOP_BASE_URL=${KUBEOP_BASE_URL}
ALLOW_INSECURE_HTTP=${ALLOW_INSECURE_HTTP}
LOGS_ROOT=${LOGS_ROOT}
LOG_DIR=${LOG_DIR}
ADMIN_JWT_SECRET=${ADMIN_JWT_SECRET}
KCFG_ENCRYPTION_KEY=${KCFG_ENCRYPTION_KEY}
PAAS_DOMAIN=${PAAS_DOMAIN}
PAAS_WILDCARD_ENABLED=${PAAS_WILDCARD_ENABLED}
ENABLE_CERT_MANAGER=${ENABLE_CERT_MANAGER}
DNS_PROVIDER=${DNS_PROVIDER}
CLOUDFLARE_API_TOKEN=${CLOUDFLARE_API_TOKEN}
CLOUDFLARE_ZONE_ID=${CLOUDFLARE_ZONE_ID}
WATCH_NAMESPACE_PREFIXES=${WATCH_NAMESPACE_PREFIXES}
WATCHER_NAMESPACE=${WATCHER_NAMESPACE}
WATCHER_WAIT_FOR_READY=true
K8S_EVENTS_BRIDGE=${K8S_EVENTS_BRIDGE}
PROJECTS_IN_USER_NAMESPACE=${PROJECTS_IN_USER_NAMESPACE}
ENV

docker compose up -d --build
cat <<'CADDY' > Caddyfile
api.${PAAS_DOMAIN} {
    reverse_proxy 127.0.0.1:8080
}
CADDY

docker run -d --name kubeop-caddy --restart unless-stopped \
  -p 80:80 -p 443:443 \
  -v "$PWD/Caddyfile":/etc/caddy/Caddyfile \
  -v kubeop-caddy-data:/data -v kubeop-caddy-config:/config \
  caddy:2
```

Create `A/AAAA` records for `api.${PAAS_DOMAIN}` that point to this host so Caddy can obtain a valid certificate.

## 3. Verify health and version

Hit the health endpoints and capture the running version.

```bash
set -euo pipefail

curl -sS "${API_ORIGIN}/healthz"
curl -sS "${API_ORIGIN}/readyz"
curl -sS -H "Authorization: Bearer ${TOKEN}" "${API_ORIGIN}/v1/version" | jq
```

Expect `{"status":"ok"}` and `{"status":"ready"}` from the first two calls and version metadata from `/v1/version`.

## 4. Register the cluster and watch the watcher rollout

Base64-encode the admin kubeconfig, register the cluster, and wait for the watcher deployment to become Ready.

```bash
set -euo pipefail

if base64 --help 2>&1 | grep -q -- '-w'; then
  CLUSTER_KCFG_B64="$(base64 -w0 "${TARGET_KUBECONFIG}")"
else
  CLUSTER_KCFG_B64="$(base64 "${TARGET_KUBECONFIG}" | tr -d '\n')"
fi

register_payload=$(jq -n --arg name "prod-cluster" --arg b64 "$CLUSTER_KCFG_B64" '{name:$name,kubeconfig_b64:$b64}')
cluster_resp=$(curl -sS -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d "$register_payload" \
  "${API_ORIGIN}/v1/clusters")

echo "$cluster_resp" | jq
export CLUSTER_ID="$(echo "$cluster_resp" | jq -r '.id')"
export CLUSTER_NAME="$(echo "$cluster_resp" | jq -r '.name')"

curl -sS -H "Authorization: Bearer ${TOKEN}" "${API_ORIGIN}/v1/clusters/${CLUSTER_ID}/health" | jq
curl -sS -H "Authorization: Bearer ${TOKEN}" "${API_ORIGIN}/v1/clusters/health" | jq

kubectl --kubeconfig "${TARGET_KUBECONFIG}" get ns
kubectl --kubeconfig "${TARGET_KUBECONFIG}" -n "${WATCHER_NAMESPACE}" rollout status deploy/"${WATCHER_DEPLOYMENT_NAME}" --timeout=3m
kubectl --kubeconfig "${TARGET_KUBECONFIG}" -n "${WATCHER_NAMESPACE}" logs deploy/"${WATCHER_DEPLOYMENT_NAME}" --tail=20
```

`/v1/clusters/{id}/health` returns `{"healthy":true}` once kubeOP can reach the API server. The Kubernetes commands confirm the watcher deployment succeeded and is streaming events.

## 5. Bootstrap the first tenant namespace

Create a user, capture the managed namespace, and verify baseline quotas and limits.

```bash
set -euo pipefail

bootstrap_payload=$(jq -n --arg name "Alice" --arg email "alice@example.com" --arg cluster "${CLUSTER_ID}" '{name:$name,email:$email,clusterId:$cluster}')
bootstrap_resp=$(curl -sS -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d "$bootstrap_payload" \
  "${API_ORIGIN}/v1/users/bootstrap")

echo "$bootstrap_resp" | jq
export USER_ID="$(echo "$bootstrap_resp" | jq -r '.user.id')"
export USER_NAMESPACE="$(echo "$bootstrap_resp" | jq -r '.namespace')"
export USER_KUBECONFIG_B64="$(echo "$bootstrap_resp" | jq -r '.kubeconfig_b64')"

python - <<'PY'
import base64, os
cfg = base64.b64decode(os.environ['USER_KUBECONFIG_B64'])
with open('alice.kubeconfig', 'wb') as fh:
    fh.write(cfg)
PY
chmod 600 alice.kubeconfig

kubectl --kubeconfig "${TARGET_KUBECONFIG}" get ns "${USER_NAMESPACE}"
kubectl --kubeconfig "${TARGET_KUBECONFIG}" -n "${USER_NAMESPACE}" get resourcequota tenant-quota -o json | jq '.spec.hard'
kubectl --kubeconfig "${TARGET_KUBECONFIG}" -n "${USER_NAMESPACE}" get limitrange tenant-limits -o json | jq '.spec.limits'
```

## 6. Mint and rotate kubeconfigs

Ensure the user binding exists, rotate credentials, and confirm namespace-scoped RBAC.

```bash
set -euo pipefail

ensure_payload=$(jq -n --arg user "${USER_ID}" --arg cluster "${CLUSTER_ID}" '{userId:$user,clusterId:$cluster}')
ensure_resp=$(curl -sS -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d "$ensure_payload" \
  "${API_ORIGIN}/v1/kubeconfigs")

echo "$ensure_resp" | jq
export BINDING_ID="$(echo "$ensure_resp" | jq -r '.id')"

rotate_payload=$(jq -n --arg id "${BINDING_ID}" '{id:$id}')
rotate_resp=$(curl -sS -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d "$rotate_payload" \
  "${API_ORIGIN}/v1/kubeconfigs/rotate")

echo "$rotate_resp" | jq
export USER_KUBECONFIG_B64="$(echo "$rotate_resp" | jq -r '.kubeconfig_b64')"
python - <<'PY'
import base64, os
cfg = base64.b64decode(os.environ['USER_KUBECONFIG_B64'])
with open('alice.kubeconfig', 'wb') as fh:
    fh.write(cfg)
PY
chmod 600 alice.kubeconfig

kubectl --kubeconfig alice.kubeconfig auth can-i create deployments -n "${USER_NAMESPACE}"
kubectl --kubeconfig alice.kubeconfig auth can-i get secrets -n "${USER_NAMESPACE}"
```

## 7. Create a project workspace

Provision a project namespace and capture the project-scoped kubeconfig.

```bash
set -euo pipefail

project_payload=$(jq -n --arg user "${USER_ID}" --arg cluster "${CLUSTER_ID}" --arg name "hello-world" '{userId:$user,clusterId:$cluster,name:$name}')
project_resp=$(curl -sS -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d "$project_payload" \
  "${API_ORIGIN}/v1/projects")

echo "$project_resp" | jq
export PROJECT_ID="$(echo "$project_resp" | jq -r '.project.id')"
export PROJECT_NAMESPACE="$(echo "$project_resp" | jq -r '.project.namespace')"
export PROJECT_KUBECONFIG_B64="$(echo "$project_resp" | jq -r '.kubeconfig_b64')"

python - <<'PY'
import base64, os
cfg = base64.b64decode(os.environ['PROJECT_KUBECONFIG_B64'])
with open('project-admin.kubeconfig', 'wb') as fh:
    fh.write(cfg)
PY
chmod 600 project-admin.kubeconfig

curl -sS -H "Authorization: Bearer ${TOKEN}" "${API_ORIGIN}/v1/projects/${PROJECT_ID}" | jq
kubectl --kubeconfig "${TARGET_KUBECONFIG}" get ns "${PROJECT_NAMESPACE}"
kubectl --kubeconfig "${TARGET_KUBECONFIG}" -n "${PROJECT_NAMESPACE}" get resourcequota tenant-quota -o json | jq '.spec.hard'
```

## 8. Prepare configmaps and secrets

Seed runtime configuration for later attachment to workloads.

```bash
set -euo pipefail

cfg_payload=$(jq -n '{name:"app-settings",data:{WELCOME_MESSAGE:"Hello from kubeOP"}}')
curl -sS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d "$cfg_payload" \
  "${API_ORIGIN}/v1/projects/${PROJECT_ID}/configs" | jq

secret_payload=$(jq -n '{name:"app-secret",stringData:{API_KEY:"super-secret"}}')
curl -sS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d "$secret_payload" \
  "${API_ORIGIN}/v1/projects/${PROJECT_ID}/secrets" | jq

kubectl --kubeconfig "${TARGET_KUBECONFIG}" -n "${PROJECT_NAMESPACE}" get configmap,secret
```

## 9. Deploy nginx with automatic DNS and TLS

Deploy an ingress-backed nginx container. kubeOP generates a wildcard hostname, provisions TLS via cert-manager, and updates Cloudflare once the Service exposes a load balancer IP.

```bash
set -euo pipefail

app_payload=$(jq -n --arg repo "https://github.com/example-org/kubeop-nginx" --arg secret "$(openssl rand -hex 16)" '{
  name: "hello-nginx",
  image: "ghcr.io/nginxinc/nginx-unprivileged:stable",
  ports: [{containerPort: 8080, servicePort: 80}],
  env: {PORT: "8080"},
  repo: $repo,
  webhookSecret: $secret
}')
app_resp=$(curl -sS -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d "$app_payload" \
  "${API_ORIGIN}/v1/projects/${PROJECT_ID}/apps")

echo "$app_resp" | jq
export APP_ID="$(echo "$app_resp" | jq -r '.appId')"

sleep 5
app_status=$(curl -sS -H "Authorization: Bearer ${TOKEN}" "${API_ORIGIN}/v1/projects/${PROJECT_ID}/apps/${APP_ID}")
echo "$app_status" | jq
export APP_FQDN="$(echo "$app_status" | jq -r '.domains[0].fqdn')"

kubectl --kubeconfig "${TARGET_KUBECONFIG}" -n "${PROJECT_NAMESPACE}" get deploy,svc,ingress
kubectl --kubeconfig "${TARGET_KUBECONFIG}" -n "${PROJECT_NAMESPACE}" rollout status deploy/hello-nginx --timeout=2m
kubectl --kubeconfig "${TARGET_KUBECONFIG}" -n "${PROJECT_NAMESPACE}" get certificate
export APP_LB_IP="$(kubectl --kubeconfig "${TARGET_KUBECONFIG}" -n "${PROJECT_NAMESPACE}" get svc hello-nginx -o jsonpath='{.status.loadBalancer.ingress[0].ip}')"

until curl -fsS -H "Authorization: Bearer ${TOKEN}" "${API_ORIGIN}/v1/projects/${PROJECT_ID}/apps/${APP_ID}" | jq -e '.domains[0].certificateStatus == "issued"' >/dev/null; do
  sleep 10
done
```

Validate HTTPS once the certificate is issued. Use `--resolve` while DNS propagates.

```bash
set -euo pipefail

curl -Ik "https://${APP_FQDN}"
if [ -n "${APP_LB_IP}" ]; then
  curl -Ik --resolve "${APP_FQDN}:443:${APP_LB_IP}" "https://${APP_FQDN}"
fi
```

## 10. Attach and detach runtime configuration

Mount the ConfigMap and Secret into the deployment, verify via `kubectl exec`, then cleanly detach both resources.

```bash
set -euo pipefail

attach_cfg_payload=$(jq -n '{name:"app-settings"}')
curl -sS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d "$attach_cfg_payload" \
  "${API_ORIGIN}/v1/projects/${PROJECT_ID}/apps/${APP_ID}/configs/attach" | jq

attach_secret_payload=$(jq -n '{name:"app-secret",prefix:"SECRET_"}')
curl -sS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d "$attach_secret_payload" \
  "${API_ORIGIN}/v1/projects/${PROJECT_ID}/apps/${APP_ID}/secrets/attach" | jq

POD_NAME="$(kubectl --kubeconfig "${TARGET_KUBECONFIG}" -n "${PROJECT_NAMESPACE}" get pods -l kubeop.app-id="${APP_ID}" -o jsonpath='{.items[0].metadata.name}')"
kubectl --kubeconfig "${TARGET_KUBECONFIG}" -n "${PROJECT_NAMESPACE}" exec "$POD_NAME" -- env | grep -E 'WELCOME_MESSAGE|SECRET_API_KEY'

detach_cfg_payload=$(jq -n '{name:"app-settings"}')
curl -sS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d "$detach_cfg_payload" \
  "${API_ORIGIN}/v1/projects/${PROJECT_ID}/apps/${APP_ID}/configs/detach" | jq

detach_secret_payload=$(jq -n '{name:"app-secret"}')
curl -sS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d "$detach_secret_payload" \
  "${API_ORIGIN}/v1/projects/${PROJECT_ID}/apps/${APP_ID}/secrets/detach" | jq
```

## 11. Observe status, logs, and events

Query application summaries, fetch logs, and stream project events alongside native Kubernetes events.

```bash
set -euo pipefail

curl -sS -H "Authorization: Bearer ${TOKEN}" "${API_ORIGIN}/v1/projects/${PROJECT_ID}/apps" | jq
curl -sS -H "Authorization: Bearer ${TOKEN}" "${API_ORIGIN}/v1/projects/${PROJECT_ID}/apps/${APP_ID}" | jq

curl -sS -H "Authorization: Bearer ${TOKEN}" "${API_ORIGIN}/v1/projects/${PROJECT_ID}/logs?tail=200"
curl -sS -H "Authorization: Bearer ${TOKEN}" "${API_ORIGIN}/v1/projects/${PROJECT_ID}/apps/${APP_ID}/logs?tailLines=100&follow=false"

kubectl --kubeconfig "${TARGET_KUBECONFIG}" -n "${PROJECT_NAMESPACE}" get events --sort-by=.lastTimestamp | tail -n10

curl -sS -H "Authorization: Bearer ${TOKEN}" "${API_ORIGIN}/v1/projects/${PROJECT_ID}/events?limit=10" | jq
custom_event_payload=$(jq -n '{kind:"deployment-note",severity:"INFO",message:"documented via API",meta:{ticket:"OPS-42"}}')
curl -sS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d "$custom_event_payload" \
  "${API_ORIGIN}/v1/projects/${PROJECT_ID}/events" | jq
```

## 12. Scale replicas and update images

Scale the deployment, update the container image, and restart the rollout while verifying readiness through Kubernetes.

```bash
set -euo pipefail

scale_payload=$(jq -n '{replicas:2}')
curl -sS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d "$scale_payload" \
  "${API_ORIGIN}/v1/projects/${PROJECT_ID}/apps/${APP_ID}/scale" | jq
kubectl --kubeconfig "${TARGET_KUBECONFIG}" -n "${PROJECT_NAMESPACE}" get deploy hello-nginx -o jsonpath='{.status.availableReplicas}' && echo

image_payload=$(jq -n '{image:"ghcr.io/nginxinc/nginx-unprivileged:alpine"}')
curl -sS -H "Authorization: Bearer ${TOKEN}" -H "Content-Type: application/json" \
  -d "$image_payload" \
  "${API_ORIGIN}/v1/projects/${PROJECT_ID}/apps/${APP_ID}/image" | jq

curl -sS -H "Authorization: Bearer ${TOKEN}" "${API_ORIGIN}/v1/projects/${PROJECT_ID}/apps/${APP_ID}/rollout/restart" | jq
kubectl --kubeconfig "${TARGET_KUBECONFIG}" -n "${PROJECT_NAMESPACE}" rollout status deploy/hello-nginx --timeout=2m
```

## 13. Deploy Helm charts and raw manifests

kubeOP can render Helm archives and apply arbitrary YAML while injecting namespace/label context.

```bash
set -euo pipefail

helm_payload=$(jq -n '{
  name: "redis-cache",
  helm: {
    chart: "https://charts.bitnami.com/bitnami/redis-18.1.3.tgz",
    values: {architecture:"standalone",auth:{enabled:false}}
  }
}')
helm_resp=$(curl -sS -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d "$helm_payload" \
  "${API_ORIGIN}/v1/projects/${PROJECT_ID}/apps")

echo "$helm_resp" | jq
export HELM_APP_ID="$(echo "$helm_resp" | jq -r '.appId')"

manifest_yaml='---
apiVersion: batch/v1
kind: Job
metadata:
  name: cleanup-job
spec:
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: curl
          image: curlimages/curl:8.11.0
          command: ["/bin/sh","-c"]
          args:
            - curl -I https://example.com
'
manifest_payload=$(jq -n --arg m "$manifest_yaml" '{name:"cleanup-job",manifests:[$m]}')
manifest_resp=$(curl -sS -H "Authorization: Bearer ${TOKEN}" \
  -H "Content-Type: application/json" \
  -d "$manifest_payload" \
  "${API_ORIGIN}/v1/projects/${PROJECT_ID}/apps")

echo "$manifest_resp" | jq
export MANIFEST_APP_ID="$(echo "$manifest_resp" | jq -r '.appId')"

kubectl --kubeconfig "${TARGET_KUBECONFIG}" -n "${PROJECT_NAMESPACE}" get deploy,sts,po,job
```

## 14. Trigger rollouts via the Git webhook

Because the nginx app was created with a repository and webhook secret, you can POST Git-style payloads to prompt redeploys.

```bash
set -euo pipefail

payload='{"ref":"refs/heads/main","repository":{"full_name":"example-org/kubeop-nginx"}}'
signature="sha256=$(printf "%s" "$payload" | openssl dgst -sha256 -hmac "$(echo "$app_payload" | jq -r '.webhookSecret')" | awk '{print $2}')"

curl -sS -H "Authorization: Bearer ${TOKEN}" \
  -H "X-Hub-Signature-256: ${signature}" \
  -H "Content-Type: application/json" \
  -d "$payload" \
  "${API_ORIGIN}/v1/webhooks/git"

kubectl --kubeconfig "${TARGET_KUBECONFIG}" -n "${PROJECT_NAMESPACE}" get deploy hello-nginx -o jsonpath='{.metadata.annotations.kubeop\.io/redeploy}' && echo
```

## 15. Cleanup and revoke access

Delete applications, revoke kubeconfigs, and remove the project and user. Validate each mutation via both the API and Kubernetes.

```bash
set -euo pipefail

curl -sS -H "Authorization: Bearer ${TOKEN}" -X DELETE "${API_ORIGIN}/v1/projects/${PROJECT_ID}/apps/${APP_ID}" | jq
curl -sS -H "Authorization: Bearer ${TOKEN}" -X DELETE "${API_ORIGIN}/v1/projects/${PROJECT_ID}/apps/${HELM_APP_ID}" | jq
curl -sS -H "Authorization: Bearer ${TOKEN}" -X DELETE "${API_ORIGIN}/v1/projects/${PROJECT_ID}/apps/${MANIFEST_APP_ID}" | jq
kubectl --kubeconfig "${TARGET_KUBECONFIG}" -n "${PROJECT_NAMESPACE}" get all

curl -sS -H "Authorization: Bearer ${TOKEN}" -X DELETE "${API_ORIGIN}/v1/kubeconfigs/${BINDING_ID}" | jq
curl -sS -H "Authorization: Bearer ${TOKEN}" -X DELETE "${API_ORIGIN}/v1/projects/${PROJECT_ID}" | jq
kubectl --kubeconfig "${TARGET_KUBECONFIG}" get ns "${PROJECT_NAMESPACE}" || echo "namespace deleted"

curl -sS -H "Authorization: Bearer ${TOKEN}" -X DELETE "${API_ORIGIN}/v1/users/${USER_ID}" | jq
kubectl --kubeconfig "${TARGET_KUBECONFIG}" get ns "${USER_NAMESPACE}" || echo "user namespace deleted"
```

`GET /v1/projects` and `GET /v1/users` should no longer list the deleted resources, and Kubernetes should report the namespaces as missing.

---

Follow these steps to bootstrap subsequent clusters, projects, and applications. Use the API and kubectl reference documents for deeper automation and troubleshooting.
