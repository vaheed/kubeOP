# Quickstart

This quickstart walks you from a fresh clone to a running kubeOP control plane in under ten minutes using Docker Compose. For
production-grade installs, review the [installation guide](INSTALL.md).

## Prerequisites

- Docker 24+
- Docker Compose v2
- `jq` and `curl`
- An administrator JWT signed with the value of `ADMIN_JWT_SECRET`

## 1. Clone and configure

```bash
git clone https://github.com/vaheed/kubeOP.git
cd kubeOP
cp docs/examples/docker-compose.env .env
mkdir -p logs
```

The `.env` file overrides defaults such as the PostgreSQL password and host paths for logs.

## 2. Launch services

```bash
docker compose up -d --build
```

Docker Compose builds the API image, provisions PostgreSQL, and mounts `./logs` for request and project event streams.

::: tip
If you prefer the published image, set `API_IMAGE=ghcr.io/vaheed/kubeop-api:latest` inside `.env`. Docker Compose will pull the
image instead of building locally.
:::

## 3. Verify readiness

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl http://localhost:8080/v1/version | jq
```

The `/v1/version` response includes build metadata, API compatibility ranges, and deprecation notices.

## 4. Authenticate once

```bash
export KUBEOP_TOKEN="<admin-jwt>"
export KUBEOP_AUTH_HEADER="-H 'Authorization: Bearer ${KUBEOP_TOKEN}'"
```

Store this snippet in your shell profile or reuse the [`_snippets/curl-headers.md`](./_snippets/curl-headers.md) fragment from the
VitePress site.

## 5. Register a cluster

Prepare a kubeconfig (base64 encoded) and call the clusters API:

```bash
B64=$(base64 -w0 < /path/to/kubeconfig)
curl -s ${KUBEOP_AUTH_HEADER} -H 'Content-Type: application/json' \
  -d "$(jq -n --arg name 'edge-cluster' --arg b64 "$B64" '{name:$name,kubeconfig_b64:$b64,"owner":"platform","environment":"staging","region":"eu-west","tags":["platform","staging"]}')" \
  http://localhost:8080/v1/clusters | jq
```

On success, the response includes the cluster ID, metadata, and `kubeop-operator` rollout status.

## 6. Bootstrap a tenant project

```bash
curl -s ${KUBEOP_AUTH_HEADER} -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"alice@example.com","clusterId":"<cluster-id>"}' \
  http://localhost:8080/v1/users/bootstrap | jq '.project'
```

kubeOP provisions namespaces, ResourceQuotas, LimitRanges, kubeconfigs, and RBAC bindings automatically.

## 7. Validate an application

```bash
curl -s ${KUBEOP_AUTH_HEADER} -H 'Content-Type: application/json' \
  -d '{"projectId":"<project-id>","name":"web","image":"ghcr.io/example/web:1.2.3","ports":[{"containerPort":80,"servicePort":80,"serviceType":"LoadBalancer"}]}' \
  http://localhost:8080/v1/apps/validate | jq '.summary'
```

The validation response lists generated Kubernetes manifests, quota usage, and SBOM digests without applying changes.

## Next steps

- Deploy the app via `POST /v1/projects/{id}/apps`
- Configure environment variables using the [configuration reference](ENVIRONMENT.md)
- Explore operational playbooks in [OPERATIONS.md](OPERATIONS.md)
