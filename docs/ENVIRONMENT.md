# Environment variables

kubeOP reads configuration from environment variables or a `.env` file. The
application first loads defaults, then an optional YAML config file (when
`CONFIG_FILE` is set), and finally environment overrides. The tables below list
the settings operators tune most often; see `.env.example` for the exhaustive
annotated list.

## Core control plane

| Variable | Purpose | Notes |
| --- | --- | --- |
| `APP_ENV` | Declares the runtime environment. | Defaults to `development`. Drives log verbosity for some scripts. |
| `PORT` | HTTP listen port for the API. | Defaults to `8080`. Docker Compose maps this to the host automatically. |
| `LOG_LEVEL` | Structured log level. | One of `debug`, `info`, `warn`, or `error`. |
| `DATABASE_URL` | PostgreSQL connection string. | Overrides individual `PG*` variables. |
| `ADMIN_JWT_SECRET` | HMAC secret used to validate admin API tokens. | Required in production unless `DISABLE_AUTH=true`. |
| `DISABLE_AUTH` | Bypass admin JWT checks. | Development-only escape hatch; never enable in production. |
| `KCFG_ENCRYPTION_KEY` | AES-256 key for stored kubeconfigs and credentials. | Required in all environments. |
| `KUBEOP_BASE_URL` | External HTTPS endpoint for kubeOP. | Used when generating callback URLs and DNS records. |
| `ALLOW_INSECURE_HTTP` | Permit `http://` base URLs. | Use only for local development or air-gapped lab setups. |
| `EVENTS_DB_ENABLED` | Toggle database-backed event timelines. | When disabled, event files under `logs/projects/<id>/events.jsonl` remain the source of truth. |

## Scheduler and automation

| Variable | Purpose | Notes |
| --- | --- | --- |
| `CLUSTER_HEALTH_INTERVAL_SECONDS` | Interval between cluster health checks. | Defaults to `60` seconds. |
| `CLUSTER_DEFAULT_ENVIRONMENT` | Default environment label for new clusters. | Applied when registration payloads omit `environment`; helps organise inventory views. |
| `CLUSTER_DEFAULT_REGION` | Default region label for new clusters. | Applied when registration payloads omit `region`; useful for reporting and filtering. |
| `PROJECTS_IN_USER_NAMESPACE` | Run projects in the user namespace. | Defaults to `true`; set to `false` for dedicated namespaces per project. |
| `MAX_LOADBALANCERS_PER_PROJECT` | Upper bound on managed `Service` objects of type `LoadBalancer`. | Defaults to `1` and protects cluster capacity. |

## Platform integration

| Variable | Purpose | Notes |
| --- | --- | --- |
| `PAAS_DOMAIN` | Base domain for application ingress. | kubeOP issues `app.project.cluster.<PAAS_DOMAIN>` hostnames when set. |
| `PAAS_WILDCARD_ENABLED` | Whether DNS automation should create wildcard records. | Works with ExternalDNS-style integrations. |
| `ENABLE_CERT_MANAGER` | Enable Cert-Manager annotations on managed ingresses. | Requires cert-manager in target clusters. |
| `LB_DRIVER` | Load balancer integration to target (`metallb`, `cilium`, or `fake`). | Defaults to `metallb`; customise to match your environment. |
| `LB_METALLB_POOL` | Address pool for MetalLB deployments. | Optional. |
| `GIT_WEBHOOK_SECRET` | Shared secret for CI webhook validation. | Required when enabling `/v1/webhooks/git`. |
| `DNS_PROVIDER` | External DNS integration (`http`, `cloudflare`, or `powerdns`). | Leave blank to disable DNS automation. |
| `DNS_API_URL` / `DNS_API_KEY` | Provider endpoint and credentials. | Used when `DNS_PROVIDER` is `http`. |
| `CLOUDFLARE_API_TOKEN` / `CLOUDFLARE_ZONE_ID` | Cloudflare token and zone configuration. | Falls back to `DNS_API_KEY` when the token is empty. |
| `PDNS_API_URL` / `PDNS_API_KEY` / `PDNS_ZONE` | PowerDNS endpoint, credentials, and zone. | Defaults the zone to `PAAS_DOMAIN` when unset. |

## Resource defaults

Namespace quotas and LimitRanges can be tuned via the `KUBEOP_DEFAULT_*` series
(e.g. `KUBEOP_DEFAULT_REQUESTS_CPU`, `KUBEOP_DEFAULT_LIMITS_MEMORY`,
`KUBEOP_DEFAULT_LR_CONTAINER_MAX_CPU`). kubeOP reapplies these values during
namespace bootstrap and reconciliation, so adjust them carefully to match
cluster capacity.

## Operational notes

- Always provide real secrets for `ADMIN_JWT_SECRET` and
  `KCFG_ENCRYPTION_KEY` in production. Inject them through GitHub Actions or
  your runtime orchestrator instead of committing values to the repository.
- Keep the database reachable before starting kubeOP; readiness checks stay in
  a failed state until migrations and connection tests succeed.
- Disable `EVENTS_DB_ENABLED` only when long-term event retention is handled by
  another system, as timelines fall back to the on-disk JSONL logs.
