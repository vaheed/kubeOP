# Install

Use this guide to deploy kubeOP for evaluation or production. Choose Docker Compose for local demos or Kubernetes for managed
control planes. The API binary runs migrations automatically on startup.

## Supported versions

| kubeOP | Managed cluster Kubernetes | PostgreSQL |
| --- | --- | --- |
| 0.14.x | 1.27 – 1.31 | 14 – 16 |

kubeOP needs network access from the API container to every managed cluster API server and to PostgreSQL. When enabling optional
DNS automation, allow outbound HTTPS to your provider (Cloudflare or PowerDNS).

## Docker Compose (local lab)

1. **Copy overrides and create logs directory**
   ```bash
   cp docs/examples/docker-compose.env .env
   mkdir -p logs
   ```
2. **Start the stack**
   ```bash
   docker compose --file docs/examples/docker-compose.yaml --env-file .env up -d --build
   ```
3. **Check status**
   ```bash
   docker compose --file docs/examples/docker-compose.yaml ps
   docker compose --file docs/examples/docker-compose.yaml logs -f api
   ```
4. **Expose the API** – The container publishes port `8080` to `localhost`. Update `.env` if you need to run on another port.
5. **Stop the stack**
   ```bash
   docker compose --file docs/examples/docker-compose.yaml down --volumes
   ```

::: caution
The sample configuration stores secrets (JWT, encryption key) in plain text for local testing. Replace them before exposing the
API outside localhost.
:::

## Kubernetes (production)

1. **Create the namespace and secrets** – Edit the sample to match your database DSN and secrets. Apply it:
   ```bash
   kubectl apply -f docs/examples/kube/kubeop-api.yaml
   ```
   The manifest creates:
   - `Namespace kubeop-system`
   - `Secret kubeop-secrets` for admin JWT and kubeconfig encryption keys
   - `Secret kubeop-postgres` containing the PostgreSQL DSN
   - `Deployment kubeop-api`
   - `Service kubeop-api`
2. **Configure PostgreSQL** – Provide a managed PostgreSQL instance reachable from the kubeOP namespace. Update the DSN in
   `kubeop-postgres` secret when credentials change.
3. **Expose the API** – Create an ingress or LoadBalancer service targeting `kubeop-api` on port 8080. Set `KUBEOP_BASE_URL`
   when publishing HTTPS endpoints.
4. **Deploy the operator** – Install the `kubeop-operator` in every managed cluster. The operator repository lives under
   `kubeOP/kubeop-operator`. Build and deploy its manager image to reconcile the App CRD.
5. **Validate health**
   ```bash
   kubectl -n kubeop-system get pods
   kubectl -n kubeop-system logs deploy/kubeop-api
   curl https://<external-host>/healthz
   ```

## Upgrade workflow

1. Read the [CHANGELOG](https://github.com/vaheed/kubeOP/blob/main/CHANGELOG.md) for breaking changes.
2. Update the Docker image tag (Docker Compose) or deployment `image` (Kubernetes).
3. Restart the API deployment. kubeOP runs database migrations on startup and logs success or failure.
4. Verify `/v1/version` returns the expected version and review logs for `database connected and migrations applied`.
5. Re-enable maintenance mode and resume normal operations (see [Operations](OPERATIONS.md)).

## Additional configuration

- Review [ENVIRONMENT.md](ENVIRONMENT.md) for every environment variable and default.
- Configure TLS termination or API gateway policies in front of the service when exposing kubeOP publicly.
- Use [SECURITY.md](SECURITY.md) to harden secrets management and incident response.
