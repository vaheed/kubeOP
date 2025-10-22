# Installation

This guide covers supported deployment options for kubeOP. Choose the path that matches your environment and follow the
configuration reference in [ENVIRONMENT.md](ENVIRONMENT.md) to customise settings.

## Support matrix

| Component | Minimum | Tested | Notes |
| --- | --- | --- | --- |
| kubeOP API | 0.11.4 | 0.11.x | Built with Go 1.24.3 |
| Kubernetes clusters | v1.26 | v1.26 – v1.30 | `kubeop-operator` reconciles `App` CRDs |
| PostgreSQL | 14 | 14, 15 | Stores metadata, events, and scheduler state |
| Docker Engine | 24 | 24.0 | Required for Docker Compose installs |
| Node.js | 20 | 20.11 | Required for VitePress docs build |

## Option A: Docker Compose

1. **Provision dependencies**
   - Install Docker and Docker Compose v2.
   - Create a persistent directory for logs: `mkdir -p /srv/kubeop/logs`.
2. **Clone and configure**
   ```bash
   git clone https://github.com/vaheed/kubeOP.git
   cd kubeOP
   cp docs/examples/docker-compose.env .env
   ```
   Edit `.env` and set `LOGS_ROOT=/srv/kubeop/logs` (or another persistent path).
3. **Start services**
   ```bash
   docker compose up -d --build
   ```
4. **Check health**
   ```bash
   curl http://localhost:8080/healthz
   curl http://localhost:8080/readyz
   ```
5. **Persist PostgreSQL data** – ensure the volume configured in `.env` points to reliable storage if you upgrade containers.

::: caution
The Docker Compose setup publishes PostgreSQL on `localhost:5432`. Restrict access with firewall rules or run Compose on a
restricted host.
:::

## Option B: Kubernetes

1. **Prepare namespace and secrets**
   ```bash
   kubectl create namespace kubeop
   kubectl create secret generic kubeop-config \
     --from-literal=ADMIN_JWT_SECRET='<random-string>' \
     --from-literal=KCFG_ENCRYPTION_KEY='<32-byte-base64>'
   ```
2. **Deploy PostgreSQL** – use your preferred operator or managed service. Record the connection string and provision backups.
3. **Apply the Deployment and Service**
   ```bash
   kubectl apply -f docs/examples/kubeop-deployment.yaml
   ```
4. **Expose the API** – integrate with your ingress controller or configure a `LoadBalancer` service.
5. **Schedule the operator** – kubeOP automatically deploys `kubeop-operator` to registered clusters. Ensure the API pod has
   network egress to the cluster API servers.

::: note
The Kubernetes manifest uses the `ghcr.io/vaheed/kubeop-api:latest` image by default. Pin a specific tag for production rollouts.
:::

## Upgrades

1. Review the [CHANGELOG](https://github.com/vaheed/kubeOP/blob/main/CHANGELOG.md) and note any breaking changes.
2. Back up the PostgreSQL database and project logs.
3. Update the image tag (`API_IMAGE` in Docker Compose or the Deployment manifest).
4. Restart the API and monitor `/readyz` until it returns HTTP 200.
5. Confirm `/v1/version` reports the expected version and compatibility range.

## Post-install validation

- `/healthz` and `/readyz` return HTTP 200.
- `/v1/version` exposes the new build metadata.
- `kubeop-operator` is deployed in each cluster registered after the upgrade (`kubectl get deployment -n kubeop-system kubeop-operator`).
