# Configuration

kubeOP is configured entirely through environment variables. The API loads
values in this order: baked-in defaults, optional YAML file (`CONFIG_FILE`), and
finally environment overrides. Copy `.env.example` when bootstrapping a new
installation—the file is fully annotated and mirrors the tables below.

## Core control plane

| Variable | Default | Purpose | Example |
| --- | --- | --- | --- |
| `ENV` | `development` | Controls logging metadata and feature toggles. | `ENV=production` |
| `PORT` | `8080` | HTTP listen port for the API. | `PORT=8443` |
| `LOG_LEVEL` | `info` | Minimum structured log level (`debug`,`info`,`warn`,`error`). | `LOG_LEVEL=debug` |
| `ALLOW_INSECURE_HTTP` | `false` | Permit HTTP base URLs for development/testing. Leave disabled in production. | `ALLOW_INSECURE_HTTP=true` |
| `DISABLE_AUTH` | `false` | Skip admin JWT enforcement (development only). | `DISABLE_AUTH=true` |
| `ADMIN_JWT_SECRET` | _(required)_ | HMAC secret for admin API tokens. | `ADMIN_JWT_SECRET=$(openssl rand -hex 32)` |
| `KCFG_ENCRYPTION_KEY` | _(required)_ | AES-GCM key for encrypting stored kubeconfigs. | `KCFG_ENCRYPTION_KEY=$(openssl rand -hex 32)` |
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/kubeop?sslmode=disable` | PostgreSQL connection string. | `DATABASE_URL=postgres://user:pass@postgres:5432/kubeop?sslmode=verify-full` |
| `EVENTS_DB_ENABLED` | `true` | Persist project events to PostgreSQL in addition to disk logs. | `EVENTS_DB_ENABLED=false` |
| `K8S_EVENTS_BRIDGE` | `false` | Enable `/v1/events/ingest` to accept batched project events from remote bridges. Alias: `EVENT_BRIDGE_ENABLED`. | `K8S_EVENTS_BRIDGE=true` |
| `CONFIG_FILE` | _empty_ | Optional YAML config path. Values inside are applied before env overrides. | `CONFIG_FILE=/etc/kubeop/config.yaml` |
| `CLUSTER_HEALTH_INTERVAL_SECONDS` | scheduler default | Override cadence of cluster health checks. | `CLUSTER_HEALTH_INTERVAL_SECONDS=300` |

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
| `LB_DRIVER` | `metallb` | Load balancer integration (`metallb`, `cilium`, or `fake`). | `LB_DRIVER=cilium` |
| `LB_METALLB_POOL` | _empty_ | MetalLB address pool for LoadBalancer Services. | `LB_METALLB_POOL=public-pool` |
| `MAX_LOADBALANCERS_PER_PROJECT` | `1` | Cap on LoadBalancer Services per project. | `MAX_LOADBALANCERS_PER_PROJECT=3` |
| `DNS_PROVIDER` | _empty_ | Select DNS automation driver (`http`, `cloudflare`, `powerdns`). Blank disables automation. | `DNS_PROVIDER=cloudflare` |
| `DNS_API_URL` | _empty_ | Base URL for the HTTP/PowerDNS providers. Cloudflare defaults to `https://api.cloudflare.com/client/v4`. | `DNS_API_URL=https://dns.example.com/v1` |
| `DNS_API_KEY` | _empty_ | Shared bearer token for the HTTP provider. Also used by Cloudflare/PowerDNS when provider-specific secrets are unset. | `DNS_API_KEY=super-secret` |
| `DNS_RECORD_TTL` | `300` | TTL applied to managed DNS A/AAAA records. | `DNS_RECORD_TTL=120` |
| `CLOUDFLARE_API_BASE` | `https://api.cloudflare.com/client/v4` | Override Cloudflare API base URL. | `CLOUDFLARE_API_BASE=https://api.cloudflare.com/client/v4` |
| `CLOUDFLARE_API_TOKEN` | _empty_ | Cloudflare API token (falls back to `DNS_API_KEY`). | `CLOUDFLARE_API_TOKEN=cf-secret` |
| `CLOUDFLARE_ZONE_ID` | _empty_ | Cloudflare zone identifier hosting the platform domain. | `CLOUDFLARE_ZONE_ID=abcd1234` |
| `PDNS_API_URL` | _empty_ | PowerDNS API base URL (defaults to `DNS_API_URL`). | `PDNS_API_URL=https://pdns.internal/api/v1` |
| `PDNS_API_KEY` | _empty_ | PowerDNS API key (defaults to `DNS_API_KEY`). | `PDNS_API_KEY=pdns-secret` |
| `PDNS_SERVER_ID` | `localhost` | PowerDNS server identifier. | `PDNS_SERVER_ID=prod-dns` |
| `PDNS_ZONE` | _empty_ | PowerDNS zone to patch (defaults to `PAAS_DOMAIN`). | `PDNS_ZONE=example.com.` |

When `PAAS_DOMAIN` and matching DNS credentials are configured, kubeOP derives
an app FQDN as `<app-full>.<project>.<cluster>.<PAAS_DOMAIN>`, requests a Let’s
Encrypt certificate via cert-manager (`letsencrypt-prod` ClusterIssuer), and
manages DNS records against the selected provider. `<app-full>` combines the
slugified app name with a deterministic short hash of the app ID so every
deployment receives a unique, stable hostname (for example,
`web-02-f7f88c5b4-4ldbq`).

## Tenancy defaults

Namespace scaffolding combines ResourceQuota and LimitRange templates. Override
values to adjust tenant guardrails.

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
| `KUBEOP_DEFAULT_SCOPES` | `NotBestEffort` | ResourceQuota scope selector. Scopes incompatible with configured resources are dropped automatically during namespace bootstrap. |
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
| `KUBEOP_DEFAULT_LR_EXT_MAX` | _empty_ | Optional pod-level max overrides (e.g. GPU extended resources). |
| `KUBEOP_DEFAULT_LR_EXT_MIN` | _empty_ | Optional pod-level min overrides. |
| `KUBEOP_DEFAULT_LR_EXT_DEFAULT` | _empty_ | Optional pod-level defaults. |
| `KUBEOP_DEFAULT_LR_EXT_DEFAULTREQUEST` | _empty_ | Optional pod-level default requests. |
| `PROJECT_LR_REQUEST_CPU` | _empty_ | Project-specific min CPU request (≤ namespace default). |
| `PROJECT_LR_REQUEST_MEMORY` | _empty_ | Project-specific min memory request. |
| `PROJECT_LR_LIMIT_CPU` | _empty_ | Project-specific CPU limit. |
| `PROJECT_LR_LIMIT_MEMORY` | _empty_ | Project-specific memory limit. |

### Pod security and networking selectors

| Variable | Default | Purpose |
| --- | --- | --- |
| `POD_SECURITY_LEVEL` | `baseline` | Pod Security Admission profile applied to namespaces. |
| `POD_SECURITY_WARN_LEVEL` | same as `POD_SECURITY_LEVEL` | Pod Security profile that triggers admission warnings. |
| `POD_SECURITY_AUDIT_LEVEL` | same as `POD_SECURITY_LEVEL` | Pod Security profile recorded in the audit backend. |
| `DNS_NAMESPACE_LABEL_KEY` | `kubernetes.io/metadata.name` | Label selector for DNS namespace lookup. |
| `DNS_NAMESPACE_LABEL_VALUE` | `kube-system` | Value for namespace label used by DNS automation. |
| `DNS_POD_LABEL_KEY` | `k8s-app` | Label key for DNS pod selection. |
| `DNS_POD_LABEL_VALUE` | `kube-dns` | Label value for DNS pod selection. |
| `INGRESS_NAMESPACE_LABEL_KEY` | `kubeop.io/ingress` | Namespace label key to detect ingress-enabled namespaces. |
| `INGRESS_NAMESPACE_LABEL_VALUE` | `true` | Namespace label value for ingress enablement. |

## Operational notes

- `ADMIN_JWT_SECRET` and `KCFG_ENCRYPTION_KEY` are mandatory in production. CI
  should inject secrets rather than committing values.
- Namespace quotas and limit ranges should be tuned alongside cluster capacity.
  kubeOP reapplies managed objects during suspend/resume and quota patches, so
  manual drift is corrected automatically.
- When disabling `EVENTS_DB_ENABLED`, disk-backed JSONL files under
  `logs/projects/<id>/events.jsonl` remain the source of truth for project
  timelines.
- Enable `K8S_EVENTS_BRIDGE` only when a trusted collector forwards
  Kubernetes events into kubeOP. Each batch response includes total,
  accepted, dropped, and error indexes for monitoring.
