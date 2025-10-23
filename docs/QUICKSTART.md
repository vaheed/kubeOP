# Quickstart

This guide gets kubeOP running on your workstation in about 10 minutes using Docker Compose. The API listens on
`http://localhost:8080` and stores data in a local PostgreSQL container.

::: include docs/_snippets/docker-compose-prereqs.md
:::

## 1. Clone the repository

```bash
git clone https://github.com/vaheed/kubeOP.git
cd kubeOP
```

## 2. Configure environment overrides

Copy the sample overrides and create a logs directory.

```bash
cp docs/examples/docker-compose.env .env
mkdir -p logs
```

The `.env` file sets the database DSN, JWT secret, and encryption key used to secure kubeconfigs.

## 3. Launch Docker Compose

```bash
docker compose --file docs/examples/docker-compose.yaml --env-file .env up -d --build
```

Docker pulls PostgreSQL and the kubeOP API image (or builds from source) and streams logs to `./logs`.

::: tip
Run `docker compose ps` to confirm both services are healthy. The API waits for PostgreSQL before starting.
:::

## 4. Verify health endpoints

```bash
curl http://localhost:8080/healthz
curl http://localhost:8080/readyz
curl http://localhost:8080/v1/version | jq
```

The `/v1/version` response includes the current semantic version, commit, and build date baked into the binary.

## 5. Authenticate once

Set an administrator JWT (the sample secret signs `{ "role": "admin" }`).

::: include docs/_snippets/curl-auth.md
:::

## 6. Register a cluster

Provide a kubeconfig so kubeOP can reach the target cluster. The helper script encodes your kubeconfig, attaches metadata, and
submits the request.

```bash
./docs/examples/curl/register-cluster.sh
```

The output contains the cluster ID required for later API calls.

## 7. Bootstrap a tenant

Create the first user and default project on the registered cluster.

```bash
curl -sS "${AUTH_HEADER[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"name":"Alice","email":"alice@example.com","clusterId":"<cluster-id>"}' \
  http://localhost:8080/v1/users/bootstrap | jq
```

The response includes the generated project ID and a short-lived kubeconfig reference for the tenant namespace.

## 8. Validate an application

Dry-run an application deployment from a container image. kubeOP renders Kubernetes manifests, evaluates quotas, and surfaces
labels and SBOM metadata.

```bash
curl -sS "${AUTH_HEADER[@]}" \
  -H 'Content-Type: application/json' \
  -d '{"projectId":"<project-id>","name":"web","image":"ghcr.io/example/web:1.2.3","ports":[{"containerPort":80,"servicePort":80,"serviceType":"LoadBalancer"}]}' \
  http://localhost:8080/v1/apps/validate | jq '.summary'
```

To commit the deployment, send the same payload to `/v1/projects/<project-id>/apps`.

## Next steps

- Follow the [Install guide](INSTALL.md) to run kubeOP on Kubernetes.
- Review the [API reference](API.md) for all endpoints and payloads.
- Read the [Operations guide](OPERATIONS.md) for backups, upgrades, and observability once you plan a production rollout.
