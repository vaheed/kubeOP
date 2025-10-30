---
outline: deep
---

# Operations

The table below captures day-2 procedures required to run kubeOP in production. Commands assume a shell with access to the target cluster and the manager API.

## Upgrades

Before cutting a release, verify the binaries that will be shipped:

```bash
go run ./tools/buildmeta --format=names
```

CI builds every discovered service (manager, operator, admission, and future additions) with branch-aware tags. When performing manual rollouts you can still promote individual images:

1. **Manager image bump**
   ```bash
   export VERSION=$(cat VERSION)
   docker buildx build \
     --platform linux/amd64,linux/arm64 \
     --file deploy/Dockerfile.manager \
     --build-arg VERSION=$VERSION --build-arg VCS_REF=$(git rev-parse --short HEAD) \
     --tag ghcr.io/vaheed/kubeop/manager:$VERSION \
     --push .
   # ensure a builder is configured (docker buildx create --use) before running the command above
 kubectl -n kubeop-system set image deploy/kubeop-manager manager=ghcr.io/vaheed/kubeop/manager:$VERSION
 kubectl -n kubeop-system rollout status deploy/kubeop-manager --timeout=180s
  ```
  For Compose-based control planes, update `KUBEOP_MANAGER_IMAGE` and `KUBEOP_OPERATOR_IMAGE` in the environment (or `.env`)
  before running `docker compose up -d` so bootstrap flows pull the desired tags with `pull_policy: always`.
2. **Operator + admission chart**
   ```bash
  helm upgrade kubeop-operator charts/kubeop-operator \
    --namespace kubeop-system \
    --set image.tag=$VERSION --set admission.image.tag=$VERSION
  ```
  The manager also honours `KUBEOP_OPERATOR_IMAGE`/`KUBEOP_OPERATOR_IMAGE_PULL_POLICY`; set them to
  `ghcr.io/vaheed/kubeop/operator:$VERSION` and `Always` when registering clusters to ensure the bootstrapper deploys the
  matching release. Development branches publish testing images at `ghcr.io/vaheed/kubeop/operator-dev:dev` when you need to
  validate unreleased changes.
3. **Post-upgrade verification**
   ```bash
   kubectl -n kubeop-system get deploy -o wide
   kubectl -n kubeop-system logs deploy/kubeop-operator | tail
   curl -s http://kubeop-manager.kubeop-system.svc.cluster.local:8080/healthz
   ```

## Backups & migrations

- **Database** – Use logical dumps during low traffic windows:
  ```bash
  kubectl -n kubeop-system exec deploy/kubeop-manager -- \
    pg_dump --dbname "$KUBEOP_DB_URL" --no-owner > kubeop-$(date +%Y%m%d).sql
  ```
- **Migrations** – Trigger the embedded Job (runs the manager image with
  `args: ["migrate"]`) after image upgrades:
  ```bash
  kubectl -n kubeop-system apply -f deploy/k8s/manager/migrations-job.yaml
  kubectl -n kubeop-system wait --for=condition=complete job/kubeop-manager-migrations --timeout=300s
  ```

## Secrets & KMS

- **Master key rotation** – Generate a new key and restart manager pods:
  ```bash
  NEW_KEY=$(openssl rand -base64 32)
  kubectl -n kubeop-system set env secret/kubeop-manager-env KUBEOP_KMS_MASTER_KEY="$NEW_KEY"
  kubectl -n kubeop-system rollout restart deploy/kubeop-manager
  ```
- **JWT signing key** – Rotate in tandem with client deployments and re-issue tokens.
- Secrets are envelope encrypted via `internal/kms` before writing to Postgres, ensuring no raw kubeconfigs persist.

## RBAC & multi-tenant policies

- **Manager roles** – JWT tokens must carry `role=admin` for write operations; unauthenticated requests receive `401`.
- **Admission enforcement** – `cmd/admission` ensures tenants/projects carry ownership labels, quotas, and namespace alignment. Webhooks are deployed with HA (`replicaCount=2`) and a PodDisruptionBudget from the Helm chart.
![Admission Webhooks](./diagrams/admission-webhooks.svg)
- **NetworkPolicy baseline** – Operator reconciliation stamps `NetworkPolicy/kubeop-egress` into every project namespace with DNS + allow-listed CIDRs sourced from `KUBEOP_EGRESS_BASELINE`.
- **ResourceQuota enforcement** – Tenant and project quotas are validated via `internal/platform/validation` and server-side apply ensures they remain in sync.

![Tenant Networking](./diagrams/tenant-networking.svg)

## Backups of cluster assets

- **CRDs & manifests** – All manifests live under `deploy/k8s/` and are embedded into the manager binary (`deploy/k8s/assets.go`). Re-run `make operator-up` after disasters.
- **Hooks** – App hooks run as Jobs. Inspect events via `kubectl -n <namespace> get events --sort-by=.lastTimestamp`.

## Disaster recovery

1. Restore Postgres from backups.
2. Reinstall kubeOP components using `make operator-up` or the Helm chart.
3. Re-seed manager secrets (`KUBEOP_KMS_MASTER_KEY`, `KUBEOP_JWT_SIGNING_KEY`).
4. Restart manager pods to re-bootstrap clusters via `/v1/clusters` reconcile loop.

## Monitoring & alerting

- **Metrics** – `/metrics` endpoints exposed by manager, operator, and admission include Prometheus counters and histograms. Scrape via ServiceMonitor (example manifests under `deploy/k8s/samples/`).
- **Logging** – All components emit structured logs (`log` package) prefixed with component names.
- **Alerting hooks** – Add alerts for:
  - `http_request_errors_total` > 0 on manager API.
  - Operator reconciliation errors (grep for `"error"` in logs).
  - Admission webhook deployment replicas below desired.

## Reference architecture

![Architecture Overview](./diagrams/arch-overview.svg)

The architecture diagram shows the manager API, PostgreSQL, operator controllers, admission webhooks, and tenant workloads communicating over the cluster control-plane network.
