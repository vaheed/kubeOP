# Configuration

All kubeOP behaviour is driven by environment variables. Values can come from `.env`, Docker Compose, Kubernetes manifests, or CI secrets. Defaults below reflect the logic in `internal/config/config.go` unless overridden by configuration files or environment variables.

## Core control plane

| Variable | Default | Purpose | Example |
| --- | --- | --- | --- |
| `ENV` | `development` | Controls logging metadata and feature toggles. | `ENV=production` |
| `PORT` | `8080` | HTTP listen port for the API. | `PORT=8443` |
| `LOG_LEVEL` | `info` | Minimum structured log level (`debug`,`info`,`warn`,`error`). | `LOG_LEVEL=debug` |
| `KUBEOP_BASE_URL` | _empty_ | External HTTPS URL for kubeOP. Powers watcher handshake + event ingest. Enables watcher auto-deploy when set. | `KUBEOP_BASE_URL=https://kubeop.example.com` |
| `ALLOW_INSECURE_HTTP` | `false` | Permit HTTP base URLs for development/testing. Leave disabled in production. | `ALLOW_INSECURE_HTTP=true` |
| `DISABLE_AUTH` | `false` | Skip admin JWT enforcement (development only). | `DISABLE_AUTH=true` |
| `ADMIN_JWT_SECRET` | _(required unless auth disabled)_ | HS256 secret for admin tokens and watcher JWT minting. | `ADMIN_JWT_SECRET=$(openssl rand -hex 32)` |
| `KCFG_ENCRYPTION_KEY` | `dev-not-secure-key` | AES-GCM key for encrypting stored kubeconfigs. Override in production. | `KCFG_ENCRYPTION_KEY=$(openssl rand -hex 32)` |
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/kubeop?sslmode=disable` | PostgreSQL connection string. | `DATABASE_URL=postgres://user:pass@postgres:5432/kubeop?sslmode=verify-full` |
| `EVENTS_DB_ENABLED` | `true` | Persist project events to PostgreSQL in addition to disk logs. | `EVENTS_DB_ENABLED=false` |
| `K8S_EVENTS_BRIDGE` | `false` | Toggle ingestion of Kubernetes core/v1 Events via watcher bridge. | `K8S_EVENTS_BRIDGE=true` |
| `CONFIG_FILE` | _empty_ | Optional YAML config path. Values inside are applied before env overrides. | `CONFIG_FILE=/etc/kubeop/config.yaml` |
| `CLUSTER_HEALTH_INTERVAL_SECONDS` | `0` (scheduler default) | Override cadence of cluster health checks. | `CLUSTER_HEALTH_INTERVAL_SECONDS=300` |

## Security and webhooks

| Variable | Default | Purpose | Example |
| --- | --- | --- | --- |
| `GIT_WEBHOOK_SECRET` | _empty_ | Shared secret to validate `/v1/webhooks/git` payloads. | `GIT_WEBHOOK_SECRET=$(openssl rand -hex 16)` |
| `ENABLE_CERT_MANAGER` | `false` | When true, app ingress rendering assumes cert-manager is present and annotates ingresses. | `ENABLE_CERT_MANAGER=true` |

## Ingress, load balancer, and DNS

| Variable | Default | Purpose | Example |
| --- | --- | --- | --- |
| `PAAS_DOMAIN` | _empty_ | Optional platform domain used for default ingress host generation. | `PAAS_DOMAIN=apps.example.com` |
| `PAAS_WILDCARD_ENABLED` | `false` | Allow wildcard ingress hosts on generated domains. | `PAAS_WILDCARD_ENABLED=true` |
| `LB_DRIVER` | `metallb` | Load balancer integration (`metallb`, custom drivers). | `LB_DRIVER=metallb` |
| `LB_METALLB_POOL` | _empty_ | MetalLB address pool for LoadBalancer Services. | `LB_METALLB_POOL=public-pool` |
| `MAX_LOADBALANCERS_PER_PROJECT` | `1` | Cap on LoadBalancer Services per project. | `MAX_LOADBALANCERS_PER_PROJECT=3` |
| `EXTERNAL_DNS_PROVIDER` | _empty_ | `cloudflare`, `powerdns`, or empty to disable DNS automation. | `EXTERNAL_DNS_PROVIDER=cloudflare` |
| `EXTERNAL_DNS_TTL` | `300` | TTL applied to managed DNS records. | `EXTERNAL_DNS_TTL=120` |
| `CF_API_TOKEN` | _empty_ | Cloudflare API token for DNS automation. | `CF_API_TOKEN=...` |
| `CF_ZONE_ID` | _empty_ | Cloudflare zone identifier. | `CF_ZONE_ID=abc123` |
| `PDNS_API_URL` | _empty_ | PowerDNS API endpoint. | `PDNS_API_URL=https://pdns.example.com/api/v1` |
| `PDNS_API_KEY` | _empty_ | PowerDNS API key. | `PDNS_API_KEY=secret` |
| `PDNS_SERVER_ID` | _empty_ | PowerDNS server identifier. | `PDNS_SERVER_ID=localhost` |
| `PDNS_ZONE` | defaults to `PAAS_DOMAIN` | PowerDNS zone when automation is enabled. | `PDNS_ZONE=apps.example.com.` |

## Tenancy defaults

Namespace scaffolding combines ResourceQuota and LimitRange templates. Override values to adjust tenant guardrails.

| Variable | Default | Purpose |
| --- | --- | --- |
| `PROJECTS_IN_USER_NAMESPACE` | `true` | Create projects inside the owning user namespace instead of dedicated namespaces. |
| `KUBEOP_DEFAULT_REQUESTS_CPU` | `2` | Namespace-wide CPU requests quota (cores). |
| `KUBEOP_DEFAULT_LIMITS_CPU` | `4` | Namespace-wide CPU limit. |
| `KUBEOP_DEFAULT_REQUESTS_MEMORY` | `4Gi` | Namespace memory requests quota. |
| `KUBEOP_DEFAULT_LIMITS_MEMORY` | `8Gi` | Namespace memory limit. |
| `KUBEOP_DEFAULT_REQUESTS_EPHEMERAL` | `10Gi` | Namespace ephemeral storage requests quota. |
| `KUBEOP_DEFAULT_LIMITS_EPHEMERAL` | `20Gi` | Namespace ephemeral storage limit. |
| `KUBEOP_DEFAULT_PODS` | `30` | Max pods per namespace. |
| `KUBEOP_DEFAULT_SERVICES` | `10` | Max Services per namespace. |
| `KUBEOP_DEFAULT_SERVICES_LOADBALANCERS` | `1` | Max LoadBalancer Services. |
| `KUBEOP_DEFAULT_CONFIGMAPS` | `100` | Max ConfigMaps. |
| `KUBEOP_DEFAULT_SECRETS` | `100` | Max Secrets. |
| `KUBEOP_DEFAULT_PVCS` | `10` | Max PersistentVolumeClaims. |
| `KUBEOP_DEFAULT_REQUESTS_STORAGE` | `200Gi` | Total requested storage. |
| `KUBEOP_DEFAULT_DEPLOYMENTS_APPS` | `20` | Max Deployments. |
| `KUBEOP_DEFAULT_REPLICASETS_APPS` | `40` | Max ReplicaSets. |
| `KUBEOP_DEFAULT_STATEFULSETS_APPS` | `5` | Max StatefulSets. |
| `KUBEOP_DEFAULT_JOBS_BATCH` | `20` | Max Jobs. |
| `KUBEOP_DEFAULT_CRONJOBS_BATCH` | `10` | Max CronJobs. |
| `KUBEOP_DEFAULT_INGRESSES_NETWORKING_K8S_IO` | `10` | Max Ingresses. |
| `KUBEOP_DEFAULT_SCOPES` | `NotBestEffort` | ResourceQuota scope selector. Scopes incompatible with configured resources (for example `NotBestEffort` with workload count quotas) are dropped automatically during namespace bootstrap. |
| `KUBEOP_DEFAULT_PRIORITY_CLASSES` | _empty_ | Optional limit on priority classes. |

### LimitRange defaults

| Variable | Default | Purpose |
| --- | --- | --- |
| `KUBEOP_DEFAULT_LR_CONTAINER_MAX_CPU` | `2` | Max CPU per container. |
| `KUBEOP_DEFAULT_LR_CONTAINER_MIN_CPU` | `100m` | Min CPU per container. |
| `KUBEOP_DEFAULT_LR_CONTAINER_MAX_MEMORY` | `2Gi` | Max memory per container. |
| `KUBEOP_DEFAULT_LR_CONTAINER_MIN_MEMORY` | `128Mi` | Min memory per container. |
| `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULT_CPU` | `500m` | Default CPU limit per container. |
| `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULT_MEMORY` | `512Mi` | Default memory limit per container. |
| `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULTREQUEST_CPU` | `300m` | Default CPU request per container. |
| `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULTREQUEST_MEMORY` | `256Mi` | Default memory request per container. |
| `KUBEOP_DEFAULT_LR_CONTAINER_MAX_EPHEMERAL` | `2Gi` | Max ephemeral storage per container. |
| `KUBEOP_DEFAULT_LR_CONTAINER_MIN_EPHEMERAL` | `256Mi` | Min ephemeral storage per container. |
| `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULT_EPHEMERAL` | `512Mi` | Default ephemeral storage limit per container. |
| `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULTREQUEST_EPHEMERAL` | `512Mi` | Default ephemeral storage request per container. |
| `KUBEOP_DEFAULT_LR_EXT_MAX` | _empty_ | Optional pod-level max overrides (e.g. `example.com/device=1`; defaults empty so clusters without GPUs schedule normally). |
| `KUBEOP_DEFAULT_LR_EXT_MIN` | _empty_ | Optional pod-level min overrides. |
| `KUBEOP_DEFAULT_LR_EXT_DEFAULT` | _empty_ | Optional pod-level defaults. |
| `KUBEOP_DEFAULT_LR_EXT_DEFAULTREQUEST` | _empty_ | Optional pod-level default requests. |
| `PROJECT_LR_REQUEST_CPU` | _empty_ | Project-specific min CPU request (should be ≤ namespace default). |
| `PROJECT_LR_REQUEST_MEMORY` | _empty_ | Project-specific min memory request. |
| `PROJECT_LR_LIMIT_CPU` | _empty_ | Project-specific CPU limit. |
| `PROJECT_LR_LIMIT_MEMORY` | _empty_ | Project-specific memory limit. |

### Pod security and networking selectors

| Variable | Default | Purpose |
| --- | --- | --- |
| `POD_SECURITY_LEVEL` | `baseline` | Pod Security Admission profile applied to namespaces. |
| `DNS_NAMESPACE_LABEL_KEY` | `kubernetes.io/metadata.name` | Label selector for DNS namespace lookup. |
| `DNS_NAMESPACE_LABEL_VALUE` | `kube-system` | Value for namespace label used by DNS automation. |
| `DNS_POD_LABEL_KEY` | `k8s-app` | Label key for DNS pod selection. |
| `DNS_POD_LABEL_VALUE` | `kube-dns` | Label value for DNS pod selection. |
| `INGRESS_NAMESPACE_LABEL_KEY` | `kubeop.io/ingress` | Namespace label key to detect ingress-enabled namespaces. |
| `INGRESS_NAMESPACE_LABEL_VALUE` | `true` | Namespace label value for ingress enablement. |

## Watcher auto-deploy

When `KUBEOP_BASE_URL` is HTTPS and no overrides disable the feature, kubeOP auto-deploys the watcher during cluster registration. All parameters can be overridden.

| Variable | Default | Purpose |
| --- | --- | --- |
| `WATCHER_AUTO_DEPLOY` | `true` when `KUBEOP_BASE_URL` set, otherwise `false` | Enable automatic watcher rollout. |
| `WATCHER_NAMESPACE` | `kubeop-system` | Namespace to host the watcher deployment. |
| `WATCHER_NAMESPACE_CREATE` | `true` (auto-enabled when auto-deploy on) | Create the namespace if missing. |
| `WATCHER_DEPLOYMENT_NAME` | `kubeop-watcher` | Deployment name applied inside the cluster. |
| `WATCHER_SERVICE_ACCOUNT` | `kubeop-watcher` | ServiceAccount bound to RBAC roles. |
| `WATCHER_SECRET_NAME` | `kubeop-watcher` | Secret storing kubeOP token and config. |
| `WATCHER_PVC_NAME` | `<deployment>-state` | PersistentVolumeClaim for informer state DB. |
| `WATCHER_PVC_STORAGE_CLASS` | _empty_ | StorageClass override for the watcher PVC. |
| `WATCHER_PVC_SIZE` | _empty_ | Requested PVC size (e.g. `1Gi`). |
| `WATCHER_IMAGE` | `ghcr.io/vaheed/kubeop:watcher` | Container image for watcher pods. |
| `WATCHER_EVENTS_URL` | `<KUBEOP_BASE_URL>/v1/events/ingest` | HTTPS endpoint used by watchers. Must be HTTP(S) when auto-deploy is enabled; defaults to the base URL. |
| `WATCHER_TOKEN` | _empty_ | Static token override. When empty kubeOP mints per-cluster JWTs. |
| `WATCHER_BATCH_MAX` | `200` | Max events per batch forwarded by the sink. |
| `WATCHER_BATCH_WINDOW_MS` | `1000` | Time window (ms) before flushing partial batches. |
| `WATCHER_STORE_PATH` | `/var/lib/kubeop-watcher/state.db` | Path for persisted informer state (inside watcher pod). |
| `WATCHER_HEARTBEAT_MINUTES` | `0` | Optional heartbeat interval. `0` disables heartbeats. |
| `WATCHER_WAIT_FOR_READY` | `true` (auto-enabled when auto-deploying) | Wait for watcher deployment availability before returning success. |
| `WATCHER_READY_TIMEOUT_SECONDS` | `180` | Timeout for watcher readiness wait loop. |

`internal/config.Config.WatcherAutoDeployExplanation()` surfaces the source (default, env, config) and validation failures in logs during startup and cluster registration.

## Watcher binary (`cmd/kubeop-watcher`)

Set these env vars when running the watcher manually.

| Variable | Default | Purpose |
| --- | --- | --- |
| `CLUSTER_ID` | _(required)_ | kubeOP cluster identifier used for authentication and tagging events. |
| `KUBEOP_BASE_URL` | _(required)_ | Base URL for the kubeOP API; watcher derives `/v1/watchers/handshake` and `/v1/events/ingest` from this. |
| `ALLOW_INSECURE_HTTP` | `false` | Permit HTTP base URLs during development. |
| `KUBEOP_EVENTS_URL` | _deprecated_ | Legacy override for ingest endpoint; prefer `KUBEOP_BASE_URL`. |
| `KUBEOP_TOKEN` | _(required)_ | Bearer token signed by kubeOP (`GenerateWatcherToken`). |
| `KUBECONFIG` | _empty_ | Path to kubeconfig file with cluster-admin permissions. |
| `LABEL_SELECTOR` | `kubeop.project.id,kubeop.app.id,kubeop.tenant.id` | Label selector applied to watched resources. The bridge accepts both dotted and dashed label variants when correlating resources. |
| `WATCH_KINDS` | defaults to `deployments.apps,replicasets.apps,ingresses.networking.k8s.io,services,events` | Comma-separated list of resources to watch. |
| `BATCH_MAX` | `200` | Per-batch event limit (matches control-plane defaults). |
| `BATCH_WINDOW_MS` | `1000` | Flush cadence in milliseconds. |
| `HTTP_TIMEOUT_SECONDS` | `15` | HTTP client timeout for ingest requests. |
| `STORE_PATH` | `/var/lib/kubeop-watcher/state.db` | SQLite path for persisted resource versions. |
| `HEARTBEAT_MINUTES` | `0` | Optional heartbeat cadence (0 disables). |
| `LISTEN_ADDR` | `:8081` | HTTP address for `/healthz`, `/readyz`, and `/metrics`. |

## Operational notes

- kubeOP validates `KUBEOP_BASE_URL` and `WATCHER_EVENTS_URL` must use HTTPS unless `ALLOW_INSECURE_HTTP=true`. Auto-deploy fails fast when URLs are missing or insecure.
- `ADMIN_JWT_SECRET` and `KCFG_ENCRYPTION_KEY` are mandatory in production. CI should inject secrets rather than committing values.
- Namespace quotas and limit ranges should be tuned alongside cluster capacity. kubeOP reapplies managed objects during suspend/resume and quota patches, so manual drift is corrected automatically.
- When disabling `EVENTS_DB_ENABLED`, disk-backed JSONL files under `logs/projects/<id>/events.jsonl` remain the source of truth for project timelines.
