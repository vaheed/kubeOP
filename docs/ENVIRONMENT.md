# Environment reference

kubeOP reads configuration from an optional YAML file (`CONFIG_FILE`) and environment variables. Defaults match the development experience baked into `docker-compose.yml`. The tables below list every supported variable grouped by concern.

## Core runtime

| Variable | Default | Description |
| --- | --- | --- |
| `APP_ENV` | `development` | Free-form environment tag used in logs and metrics. |
| `PORT` | `8080` | HTTP listen port for the API server. |
| `LOG_LEVEL` | `info` | Global log level (`debug`, `info`, `warn`, `error`). |
| `KUBEOP_BASE_URL` | _(empty)_ | Optional external URL. Must be HTTPS unless `ALLOW_INSECURE_HTTP=true`. |
| `ALLOW_INSECURE_HTTP` | `false` | Permit `http://` schemes for `KUBEOP_BASE_URL`. |
| `CONFIG_FILE` | _(empty)_ | Path to optional YAML configuration file. |

## Authentication & secrets

| Variable | Default | Description |
| --- | --- | --- |
| `ADMIN_JWT_SECRET` | `dev-admin-secret-change-me` | HMAC secret for admin JWTs. Required unless `DISABLE_AUTH=true`. |
| `DISABLE_AUTH` | `false` | Disables authentication (development only). |
| `KCFG_ENCRYPTION_KEY` | `dev-not-secure-key` | Symmetric key for encrypting stored kubeconfigs. Required. |
| `ALLOW_GIT_FILE_PROTOCOL` | `false` | Allow `file://` Git sources for local testing. |
| `GIT_WEBHOOK_SECRET` | _(empty)_ | Shared secret used to validate incoming Git webhooks. |

## Database & events

| Variable | Default | Description |
| --- | --- | --- |
| `DATABASE_URL` | `postgres://postgres:postgres@localhost:5432/kubeop?sslmode=disable` | Connection string for the primary PostgreSQL database. |
| `EVENTS_DB_ENABLED` | `true` | Enable events persistence (defaults to true unless a config file overrides). |
| `K8S_EVENTS_BRIDGE` / `EVENT_BRIDGE_ENABLED` | `false` | Toggle for the Kubernetes events ingest endpoint. |

## Cluster health scheduler

| Variable | Default | Description |
| --- | --- | --- |
| `CLUSTER_HEALTH_INTERVAL_SECONDS` | `60` | Polling interval for the cluster health scheduler. |

## Cluster metadata defaults

| Variable | Default | Description |
| --- | --- | --- |
| `CLUSTER_DEFAULT_ENVIRONMENT` | _(empty)_ | Applied when registering clusters without an explicit environment. |
| `CLUSTER_DEFAULT_REGION` | _(empty)_ | Applied when registering clusters without an explicit region. |

## Namespace placement & pod security

| Variable | Default | Description |
| --- | --- | --- |
| `PROJECTS_IN_USER_NAMESPACE` | `true` | Place projects inside the owning user namespace instead of dedicated namespaces. |
| `POD_SECURITY_LEVEL` | `baseline` | Enforced Pod Security Admission level. |
| `POD_SECURITY_WARN_LEVEL` | `baseline` | Warning Pod Security level. |
| `POD_SECURITY_AUDIT_LEVEL` | `baseline` | Audit Pod Security level. |

## Canonical label helpers

| Variable | Default | Description |
| --- | --- | --- |
| `DNS_NS_LABEL_KEY` | `kubernetes.io/metadata.name` | Namespace label key used when searching for DNS pods. |
| `DNS_NS_LABEL_VALUE` | `kube-system` | Namespace label value used when searching for DNS pods. |
| `DNS_POD_LABEL_KEY` | `k8s-app` | Pod label key for DNS pods. |
| `DNS_POD_LABEL_VALUE` | `kube-dns` | Pod label value for DNS pods. |
| `INGRESS_NS_LABEL_KEY` | `kubeop.io/ingress` | Namespace label key that marks ingress-enabled namespaces. |
| `INGRESS_NS_LABEL_VALUE` | `true` | Namespace label value that marks ingress-enabled namespaces. |

## Namespace resource quotas

The following variables control namespace-level `ResourceQuota` defaults. Values are strings passed directly to Kubernetes quantity parsers.

| Variable | Default |
| --- | --- |
| `KUBEOP_DEFAULT_REQUESTS_CPU` | `2` |
| `KUBEOP_DEFAULT_LIMITS_CPU` | `4` |
| `KUBEOP_DEFAULT_REQUESTS_MEMORY` | `4Gi` |
| `KUBEOP_DEFAULT_LIMITS_MEMORY` | `8Gi` |
| `KUBEOP_DEFAULT_REQUESTS_EPHEMERAL` | `10Gi` |
| `KUBEOP_DEFAULT_LIMITS_EPHEMERAL` | `20Gi` |
| `KUBEOP_DEFAULT_PODS` | `30` |
| `KUBEOP_DEFAULT_SERVICES` | `10` |
| `KUBEOP_DEFAULT_SERVICES_LOADBALANCERS` | `1` |
| `KUBEOP_DEFAULT_CONFIGMAPS` | `100` |
| `KUBEOP_DEFAULT_SECRETS` | `100` |
| `KUBEOP_DEFAULT_PVCS` | `10` |
| `KUBEOP_DEFAULT_REQUESTS_STORAGE` | `200Gi` |
| `KUBEOP_DEFAULT_DEPLOYMENTS_APPS` | `20` |
| `KUBEOP_DEFAULT_REPLICASETS_APPS` | `40` |
| `KUBEOP_DEFAULT_STATEFULSETS_APPS` | `5` |
| `KUBEOP_DEFAULT_JOBS_BATCH` | `20` |
| `KUBEOP_DEFAULT_CRONJOBS_BATCH` | `10` |
| `KUBEOP_DEFAULT_INGRESSES_NETWORKING_K8S_IO` | `10` |
| `KUBEOP_DEFAULT_SCOPES` | `NotBestEffort` |
| `KUBEOP_DEFAULT_PRIORITY_CLASSES` | _(empty)_ |

## Namespace limit ranges

| Variable | Default |
| --- | --- |
| `KUBEOP_DEFAULT_LR_CONTAINER_MAX_CPU` | `2` |
| `KUBEOP_DEFAULT_LR_CONTAINER_MAX_MEMORY` | `2Gi` |
| `KUBEOP_DEFAULT_LR_CONTAINER_MIN_CPU` | `100m` |
| `KUBEOP_DEFAULT_LR_CONTAINER_MIN_MEMORY` | `128Mi` |
| `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULT_CPU` | `500m` |
| `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULT_MEMORY` | `512Mi` |
| `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULTREQUEST_CPU` | `300m` |
| `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULTREQUEST_MEMORY` | `256Mi` |
| `KUBEOP_DEFAULT_LR_CONTAINER_MAX_EPHEMERAL` | `2Gi` |
| `KUBEOP_DEFAULT_LR_CONTAINER_MIN_EPHEMERAL` | `128Mi` |
| `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULT_EPHEMERAL` | `512Mi` |
| `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULTREQUEST_EPHEMERAL` | `256Mi` |
| `KUBEOP_DEFAULT_LR_EXT_MAX` | _(empty)_ |
| `KUBEOP_DEFAULT_LR_EXT_MIN` | _(empty)_ |
| `KUBEOP_DEFAULT_LR_EXT_DEFAULT` | _(empty)_ |
| `KUBEOP_DEFAULT_LR_EXT_DEFAULTREQUEST` | _(empty)_ |

## Project-level limit ranges

| Variable | Default |
| --- | --- |
| `PROJECT_LR_REQUEST_CPU` | `100m` |
| `PROJECT_LR_REQUEST_MEMORY` | `128Mi` |
| `PROJECT_LR_LIMIT_CPU` | `1` |
| `PROJECT_LR_LIMIT_MEMORY` | `1Gi` |

## Delivery & ingress

| Variable | Default | Description |
| --- | --- | --- |
| `PAAS_DOMAIN` | _(empty)_ | Optional platform domain used when generating ingress hosts. |
| `PAAS_WILDCARD_ENABLED` | `false` | Enables wildcard ingress configuration. |
| `ENABLE_CERT_MANAGER` | `false` | Annotate ingresses for cert-manager issuance. |
| `LB_DRIVER` | `metallb` | Load balancer integration (`metallb`, etc.). |
| `LB_METALLB_POOL` | _(empty)_ | Metallb pool name to target. |
| `MAX_LOADBALANCERS_PER_PROJECT` | `1` | Hard limit for load balancer services per project. |

## External DNS automation

| Variable | Default | Description |
| --- | --- | --- |
| `DNS_PROVIDER` | _(empty)_ | `cloudflare`, `powerdns`, or custom integration hint. |
| `DNS_API_URL` | _(empty)_ | Base API URL for DNS provider. |
| `DNS_API_KEY` | _(empty)_ | Shared API key (used as a fallback for provider-specific keys). |
| `DNS_RECORD_TTL` | `300` | TTL for created DNS records. |
| `CLOUDFLARE_API_TOKEN` | _(empty)_ | Cloudflare API token (falls back to `DNS_API_KEY`). |
| `CLOUDFLARE_ZONE_ID` | _(empty)_ | Cloudflare zone ID. |
| `CLOUDFLARE_API_BASE` | `https://api.cloudflare.com/client/v4` (or `DNS_API_URL`) | Override for Cloudflare API host. |
| `PDNS_API_URL` | _(empty)_ | PowerDNS API URL (falls back to `DNS_API_URL`). |
| `PDNS_API_KEY` | _(empty)_ | PowerDNS API key (falls back to `DNS_API_KEY`). |
| `PDNS_SERVER_ID` | `localhost` | PowerDNS server identifier. |
| `PDNS_ZONE` | _(empty)_ (falls back to `PAAS_DOMAIN`) | PowerDNS zone to manage. |

## Operator deployment

| Variable | Default |
| --- | --- |
| `OPERATOR_NAMESPACE` | `kubeop-system` |
| `OPERATOR_DEPLOYMENT_NAME` | `kubeop-operator` |
| `OPERATOR_SERVICE_ACCOUNT` | `kubeop-operator` |
| `OPERATOR_IMAGE` | `ghcr.io/vaheed/kubeop-operator-manager:latest` |
| `OPERATOR_IMAGE_PULL_POLICY` | `IfNotPresent` |
| `OPERATOR_LEADER_ELECTION` | `false` |

## Filesystem layout

kubeOP writes per-project logs and delivery artifacts under `logs/<project-id>`. Ensure the process user can read and write to that directory (the Docker Compose quickstart mounts `./logs`).

## Validation guards

Configuration validation occurs after defaults are applied:

- `ADMIN_JWT_SECRET` must be non-empty when authentication is enabled.
- `KCFG_ENCRYPTION_KEY` is always required.
- `KUBEOP_BASE_URL` must use HTTPS unless `ALLOW_INSECURE_HTTP=true`.

When configuration is invalid, kubeOP terminates with a descriptive error before binding any sockets.
