# Quickstart

This walkthrough spins up kubeOP locally with Docker Compose and exercises the core API endpoints using `curl`.

## Prerequisites

- Docker and Docker Compose v2
- `curl`
- `jq`

## 1. Clone the repository

```bash
git clone https://github.com/vaheed/kubeOP.git
cd kubeOP
```

## 2. Configure environment variables

Copy the sample Compose environment and prepare log directories:

```bash
cp docs/examples/docker-compose.env .env
mkdir -p logs
```

Review `.env` and update secrets as needed (`ADMIN_JWT_SECRET`, `KCFG_ENCRYPTION_KEY`).

## 3. Launch services

```bash
docker compose up -d --build
```

- API: `http://localhost:8080`
- PostgreSQL: `postgres://postgres:postgres@localhost:5432/kubeop?sslmode=disable`

## 4. Check health

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl http://localhost:8080/v1/version | jq
```

`/v1/version` now returns only immutable build metadata (version, commit, date) after the v0.14.0 cleanup.

## 5. Authenticate

```bash
export KUBEOP_TOKEN='<admin-jwt>'
export KUBEOP_AUTH_HEADER="-H 'Authorization: Bearer ${KUBEOP_TOKEN}'"
```

## 6. Register a cluster

```bash
B64=$(base64 -w0 < /path/to/kubeconfig)
curl -s ${KUBEOP_AUTH_HEADER} -H 'Content-Type: application/json' \
  -d "$(jq -n --arg name 'edge-cluster' --arg b64 "$B64" '{name:$name,kubeconfig_b64:$b64,"owner":"platform","environment":"staging","region":"eu-west"}')" \
  http://localhost:8080/v1/clusters | jq
```

The response includes the cluster ID required for subsequent requests.

## 7. Bootstrap a user and project

```bash
curl -s ${KUBEOP_AUTH_HEADER} -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"alice@example.com","clusterId":"<cluster-id>"}' \
  http://localhost:8080/v1/users/bootstrap | jq
```

The payload returns the created user, namespace, and project IDs.

## 8. Validate an application deployment

```bash
curl -s ${KUBEOP_AUTH_HEADER} -H 'Content-Type: application/json' \
  -d '{"projectId":"<project-id>","name":"web","image":"ghcr.io/example/web:1.2.3","ports":[{"containerPort":80,"servicePort":80,"serviceType":"LoadBalancer"}]}' \
  http://localhost:8080/v1/apps/validate | jq '.summary'
```

The summary includes rendered Kubernetes objects with canonical labels (`kubeop.app.id`, `kubeop.project.id`, etc.).

## 9. Deploy the application

```bash
curl -s ${KUBEOP_AUTH_HEADER} -H 'Content-Type: application/json' \
  -d '{"projectId":"<project-id>","name":"web","image":"ghcr.io/example/web:1.2.3","ports":[{"containerPort":80,"servicePort":80,"serviceType":"LoadBalancer"}]}' \
  http://localhost:8080/v1/projects/<project-id>/apps | jq
```

The deployment triggers `kubeop-operator` to reconcile the workload inside the target cluster.

## 10. Inspect status and logs

```bash
curl -s ${KUBEOP_AUTH_HEADER} http://localhost:8080/v1/projects/<project-id>/apps/<app-id>/status | jq
curl -s ${KUBEOP_AUTH_HEADER} http://localhost:8080/v1/projects/<project-id>/logs/<app-id>?tailLines=50
```

## 11. Tear down

```bash
docker compose down -v
rm -rf logs .data
```

You now have a clean baseline for iterating on kubeOP or running the integration tests locally.
