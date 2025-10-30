---
outline: deep
---

# Getting Started

This guide covers two deployment paths:

1. **Local evaluation** using Kind for Kubernetes, Docker Compose for supporting services, and the kubeOP Helm chart.
2. **Production-like cluster installation** on an existing Kubernetes cluster with external dependencies.

All steps are reproducible through `make` targets and can be executed end-to-end without manual edits.

## Prerequisites

| Tool | Version | Purpose |
| --- | --- | --- |
| Go | 1.22+ | Build/test binaries. |
| Node.js | 18+ | Build the VitePress documentation. |
| Docker & Docker Compose | latest | Build images, run Postgres/ACME mocks. |
| Kind | ≥0.23 | Provision a local Kubernetes cluster. |
| kubectl | matching cluster | Interact with Kubernetes resources. |
| Helm | ≥3.12 | Install the kubeOP operator chart. |
| jq, yq | latest | Helpers for scripts and E2E tests. |

Clone the repository and install Node.js dependencies for documentation:

```bash
npm install
```

## Local evaluation (Kind + Compose + Helm)

The `Makefile` orchestrates the full stack. Run the following commands from the project root:

```bash
make kind-up        # Create Kind cluster using e2e/kind-config.yaml
make platform-up    # Start Postgres, ACME mocks, and provision namespaces/secrets
make manager-up     # Launch the manager API via Docker Compose
make operator-up    # Install CRDs, RBAC, and controller deployment
```
The manager automatically bootstraps each newly registered cluster with the kubeOP namespace, CRDs, RBAC, admission stack, and
the operator deployment using the GitHub Container Registry image (`ghcr.io/vaheed/kubeop/operator`) and enforces
`imagePullPolicy: Always` so the cluster always pulls the requested tag.

### Verify the platform

1. **Check core namespaces**:
   ```bash
   kubectl get ns kubeop-system kubeop-tenants
   ```
2. **Inspect services**:
   ```bash
   kubectl -n kubeop-system get deploy,sts,svc
   ```
3. **Health checks**:
   ```bash
   curl -s http://localhost:18080/healthz
   curl -s http://localhost:18080/version | jq
   kubectl -n kubeop-system get deploy/kubeop-operator -o jsonpath='{.status.readyReplicas}'
   ```

### Run the end-to-end suite

The E2E workflow boots a tenant/project/app, provisions DNS/TLS mocks, exercises policy enforcement, rollouts, rollbacks, usage, invoicing, and analytics collection.

```bash
make test-e2e
```

Artifacts are written to `./artifacts/` including bootstrap logs, E2E logs, API summaries, usage snapshots, and invoice exports.

### Tear down

```bash
make down
```

This command dumps Kind logs, deletes the cluster, and shuts down Docker Compose services.

## Deploying to an existing cluster

When running on a real cluster, reuse the same assets with the following adjustments:

1. **Bootstrap supporting services** using the production Docker Compose stack:
   ```bash
   export KUBEOP_KMS_MASTER_KEY="$(openssl rand -base64 32)"
   export KUBEOP_JWT_SIGNING_KEY="$(openssl rand -base64 32)"
   export KUBEOP_OPERATOR_IMAGE="ghcr.io/vaheed/kubeop/operator:$(cat VERSION)"
   docker compose up -d
   ```
   The compose file (`./docker-compose.yml`) provisions PostgreSQL and the manager container with `pull_policy: always` so
   deploys track the published GHCR images. Override `KUBEOP_OPERATOR_IMAGE` if you need to pin to a different tag or
   registry. Development snapshots live under `ghcr.io/vaheed/kubeop/manager-dev:dev` and `ghcr.io/vaheed/kubeop/operator-dev:dev`.
2. **Configure environment**:
   ```bash
   cp .env.example .env
   export KUBEOP_DB_URL="postgres://user:pass@db.example:5432/kubeop?sslmode=require"
   export KUBEOP_KMS_MASTER_KEY="$(openssl rand -base64 32)"
   export KUBEOP_JWT_SIGNING_KEY="$(openssl rand -base64 32)"
   export KUBEOP_REQUIRE_AUTH=true
   ```
3. **Deploy the manager**: create the namespace, secret, and deployment/service using the sample manifest below.
   ```bash
   kubectl create namespace kubeop-system
   cat <<'YAML' | kubectl -n kubeop-system apply -f -
   apiVersion: v1
   kind: Secret
   metadata:
     name: kubeop-manager-env
   stringData:
     KUBEOP_DB_URL: "$KUBEOP_DB_URL"
     KUBEOP_KMS_MASTER_KEY: "$KUBEOP_KMS_MASTER_KEY"
     KUBEOP_JWT_SIGNING_KEY: "$KUBEOP_JWT_SIGNING_KEY"
     KUBEOP_REQUIRE_AUTH: "true"
   ---
   apiVersion: apps/v1
   kind: Deployment
   metadata:
     name: kubeop-manager
     labels:
       app: kubeop-manager
   spec:
     replicas: 2
     selector:
       matchLabels:
         app: kubeop-manager
     template:
       metadata:
         labels:
           app: kubeop-manager
       spec:
         containers:
           - name: manager
            image: ghcr.io/vaheed/kubeop/manager:$(cat VERSION)
             envFrom:
               - secretRef:
                   name: kubeop-manager-env
             ports:
               - containerPort: 8080
             readinessProbe:
               httpGet:
                 path: /healthz
                 port: 8080
               periodSeconds: 10
             livenessProbe:
               httpGet:
                 path: /healthz
                 port: 8080
               periodSeconds: 20
             resources:
               requests:
                 cpu: 100m
                 memory: 256Mi
   ---
   apiVersion: v1
   kind: Service
   metadata:
     name: kubeop-manager
   spec:
     selector:
       app: kubeop-manager
     ports:
       - name: http
         port: 80
         targetPort: 8080
   YAML
   ```
4. **Install Helm chart**:
   ```bash
   helm upgrade --install kubeop-operator charts/kubeop-operator \
     --namespace kubeop-system \
     --create-namespace \
     --set manager.image.tag=$(cat VERSION)
   ```
5. **Run database migrations**:
   ```bash
   kubectl -n kubeop-system apply -f deploy/k8s/manager/migrations-job.yaml
   kubectl -n kubeop-system wait --for=condition=complete job/kubeop-manager-migrations --timeout=180s
   ```
6. **Expose the manager API** using an Ingress or LoadBalancer that targets the `kubeop-manager` service.
7. **Register the cluster**: call the manager API with a kubeconfig. Example:
   ```bash
   curl -H "Authorization: Bearer <admin-token>" \
     -H "Content-Type: application/json" \
     -X POST http://kubeop-manager.example.com/v1/clusters \
     -d "$(jq -n --arg name prod --argfile kc kubeconfig.json '{name:$name,kubeconfig:$kc}')"
   ```

## Next steps

- Continue to the [Bootstrap Guide](./bootstrap-guide.md) for tenant/project/app workflows.
- Review [Operations](./operations.md) for upgrade, backup, and policy procedures.
- Explore [Delivery](./delivery.md) for rollout strategies and rollback processes.
