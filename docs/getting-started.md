# 5-minute quickstart

This guide boots kubeOP, registers a cluster, and deploys a sample app using Docker Compose.

## Prerequisites

- Docker Engine 24+ and Docker Compose plugin.
- `curl`, `jq`, and `kubectl`.
- A base64-encoded kubeconfig for a test cluster (Talos or upstream Kubernetes). The kubeconfig must grant cluster-admin so kubeOP can create namespaces, quotas, and RBAC.

## 1. Clone and install dependencies

```bash
git clone https://github.com/vaheed/kubeOP.git
cd kubeOP
npm install
```

`npm install` installs the VitePress tooling used for documentation builds.

## 2. Configure environment

Copy the sample environment file, set a strong admin JWT secret, and enable admin bypass for the quickstart.

```bash
cp .env.example .env
export ADMIN_JWT_SECRET=$(openssl rand -hex 32)
```

Edit `.env` and update:

- `ADMIN_JWT_SECRET=<your value>`
- `DISABLE_AUTH=true` (only for local testing)

## 3. Launch kubeOP and PostgreSQL

```bash
docker compose up -d --build
```

- API listens on `http://localhost:8080`.
- PostgreSQL is exposed internally on `postgres:5432` and seeded with the `kubeop` database.
- Logs are written to `./logs` and rotated via Lumberjack.

## 4. Authenticate (optional)

When `DISABLE_AUTH=true`, skip this step. For production-like testing keep auth enabled and mint an HS256 token using the configured secret and the claim `{"role":"admin"}`. Any JWT generator works; the control plane validates signature and role.

## 5. Register a cluster

```bash
B64=$(base64 -w0 </path/to/kubeconfig)
curl -s -X POST \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer ${TOKEN:-dummy}" \
  -d "$(jq -n --arg name 'demo' --arg cfg "$B64" '{name:$name,kubeconfig_b64:$cfg,"owner":"platform","environment":"staging","region":"eu-west","tags":["platform","staging"]}')" \
  http://localhost:8080/v1/clusters | jq
```

The response returns a cluster `id` and echoes metadata (owner, environment,
region, tags). Keep the ID for later steps.

## 6. Bootstrap a user namespace

```bash
USER_PAYLOAD=$(jq -n \
  --arg name 'Alice' \
  --arg email 'alice@example.com' \
  --arg cluster "$CLUSTER_ID" \
  '{name:$name,email:$email,clusterId:$cluster}')
USER=$(curl -s -X POST \
  -H "Authorization: Bearer ${TOKEN:-dummy}" \
  -H 'Content-Type: application/json' \
  -d "$USER_PAYLOAD" \
  http://localhost:8080/v1/users/bootstrap)
USER_ID=$(echo "$USER" | jq -r '.user.id')
NAMESPACE=$(echo "$USER" | jq -r '.namespace')
```

kubeOP provisions a namespace, applies managed quota/limits, and returns a namespace-scoped kubeconfig in `kubeconfig_b64`.

## 7. Create a project and deploy an app

```bash
PROJECT_PAYLOAD=$(jq -n \
  --arg user "$USER_ID" \
  --arg cluster "$CLUSTER_ID" \
  --arg name 'hello-world' \
  '{userId:$user,clusterId:$cluster,name:$name}')
PROJECT=$(curl -s -X POST \
  -H "Authorization: Bearer ${TOKEN:-dummy}" \
  -H 'Content-Type: application/json' \
  -d "$PROJECT_PAYLOAD" \
  http://localhost:8080/v1/projects)
PROJECT_ID=$(echo "$PROJECT" | jq -r '.id')

APP_PAYLOAD=$(jq -n '{image:"nginx:1.27",name:"web",ports:[{containerPort:80,servicePort:80,protocol:"TCP",serviceType:"ClusterIP"}]}')
APP=$(curl -s -X POST \
  -H "Authorization: Bearer ${TOKEN:-dummy}" \
  -H 'Content-Type: application/json' \
  -d "$APP_PAYLOAD" \
  http://localhost:8080/v1/projects/$PROJECT_ID/apps)
APP_ID=$(echo "$APP" | jq -r '.appId')
```

## 8. Inspect rollout status

```bash
curl -s -H "Authorization: Bearer ${TOKEN:-dummy}" \
  http://localhost:8080/v1/projects/$PROJECT_ID/apps/$APP_ID | jq
```

The response reports desired/ready replicas, exposed services, and ingress hosts by querying the Kubernetes API.

## 9. Stream logs and events

```bash
curl -s -H "Authorization: Bearer ${TOKEN:-dummy}" \
  "http://localhost:8080/v1/projects/$PROJECT_ID/apps/$APP_ID/logs?tailLines=100&follow=false"

curl -s -H "Authorization: Bearer ${TOKEN:-dummy}" \
  "http://localhost:8080/v1/projects/$PROJECT_ID/events?limit=20" | jq
```

Logs stream from disk-backed files stored under `logs/projects/<project_id>/`. Events return paginated records from PostgreSQL.

## 10. Tear down

```bash
docker compose down -v
```

Volumes and logs are removed. Re-run the steps above to recreate the environment.

## Samples library

Looking for automation examples? Explore `samples/00-bootstrap` after completing the quickstart.
The scripts source shared helpers from `samples/lib/common.sh`, enforce `set -euo pipefail`, and
log every action with timestamps. Start by copying `.env.example` to `.env`, editing the required
variables, and running `./curl.sh`, `./verify.sh`, and `./cleanup.sh` to validate connectivity.
