# Installation

kubeOP ships as a Go binary with an optional Docker Compose bundle. Pick the workflow that matches your environment.

## Docker Compose (local development)

1. **Clone the repository**

   ```bash
   git clone https://github.com/vaheed/kubeOP.git
   cd kubeOP
   ```

2. **Prepare environment**

   ```bash
   cp docs/examples/docker-compose.env .env
   mkdir -p logs
   ```

3. **Launch services**

   ```bash
   docker compose up -d --build
   ```

   Services started:

   - `api` – kubeOP REST API on `http://localhost:8080`
   - `postgres` – single-node PostgreSQL with persisted volume `./.data/postgres`

4. **Tail logs**

   ```bash
   docker compose logs -f api
   ```

5. **Shut down**

   ```bash
   docker compose down
   ```

## Kubernetes deployment (bring your own platform)

1. **Build and push images**

   ```bash
   make docker-build docker-push \
     REGISTRY=ghcr.io/<org> \
     VERSION=v0.14.0
   ```

   The `Dockerfile` exposes `api` and `operator` build targets. Publish both if you plan to run the operator from the same registry.

2. **Create secrets**

   ```bash
   kubectl -n kubeop-system create secret generic kubeop-secrets \
     --from-literal=ADMIN_JWT_SECRET='<random>' \
     --from-literal=KCFG_ENCRYPTION_KEY='<random>'
   ```

3. **Deploy the API**

   Use the sample manifest at [`docs/examples/kubeop-deployment.yaml`](examples/kubeop-deployment.yaml) as a starting point. Patch the container image, environment variables, and database connection string to match your platform.

4. **Deploy the operator to each managed cluster**

   ```bash
   kubectl apply -f kubeop-operator/config/crd/bases
   kubectl apply -f kubeop-operator/config/default
   ```

   Override the manager image (via `kustomize` or Helm) to align with the version built in step 1.

5. **Expose the API securely**

   Terminate TLS at your ingress controller or external load balancer. Ensure only trusted networks can reach the `/v1/*` endpoints.

6. **Configure environment variables**

   Review [`docs/ENVIRONMENT.md`](ENVIRONMENT.md) for required settings. At minimum set `DATABASE_URL`, `ADMIN_JWT_SECRET`, and `KCFG_ENCRYPTION_KEY` in the API deployment.

## Upgrades

1. Deploy the new container images (API and operator).
2. Run database migrations (the API runs them automatically on startup).
3. Verify `/v1/version` returns the expected version (no compatibility metadata remains in v0.14.0).
4. Monitor `/metrics` and application logs for regressions.
