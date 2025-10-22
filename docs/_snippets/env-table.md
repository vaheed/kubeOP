## Core settings

| Variable | Purpose | Default | Example |
| --- | --- | --- | --- |
| `APP_ENV` | Environment label surfaced in logs and `/v1/version`. | `development` | `production` |
| `PORT` | HTTP listen port for the API. | `8080` | `8443` |
| `LOG_LEVEL` | Global log level. | `info` | `debug` |
| `KUBEOP_BASE_URL` | Public URL for generating links. Requires HTTPS unless `ALLOW_INSECURE_HTTP=true`. | _(empty)_ | `https://kubeop.example.com` |
| `ALLOW_INSECURE_HTTP` | Permit HTTP URLs in `KUBEOP_BASE_URL`. | `false` | `true` |
| `DATABASE_URL` | PostgreSQL connection string. | `postgres://postgres:postgres@localhost:5432/kubeop?sslmode=disable` | `postgres://user:pass@db:5432/kubeop?sslmode=require` |
| `CLUSTER_HEALTH_INTERVAL_SECONDS` | Polling interval for the cluster health scheduler. | `60` | `120` |

## Security & authentication

| Variable | Purpose | Default | Example |
| --- | --- | --- | --- |
| `ADMIN_JWT_SECRET` | HMAC secret for verifying admin JWTs. | `dev-admin-secret-change-me` | `$(openssl rand -hex 32)` |
| `DISABLE_AUTH` | Bypass JWT verification (development only). | `false` | `true` |
| `KCFG_ENCRYPTION_KEY` | 32-byte key for encrypting kubeconfigs. | `dev-not-secure-key` | `$(openssl rand -base64 32)` |
| `ALLOW_GIT_FILE_PROTOCOL` | Enable `file://` Git sources. | `false` | `true` |
| `GIT_WEBHOOK_SECRET` | Shared secret for webhook signature validation. | _(empty)_ | `supers3cret` |
| `HELM_CHART_ALLOWED_HOSTS` | Comma-separated list of domains allowed for Helm chart downloads. Supports `*.example.com` wildcards. | _(empty)_ | `charts.example.com,*.trusted.io` |

## Events & telemetry

| Variable | Purpose | Default | Example |
| --- | --- | --- | --- |
| `EVENTS_DB_ENABLED` | Persist events in PostgreSQL. | `true` | `false` |
| `K8S_EVENTS_BRIDGE` / `EVENT_BRIDGE_ENABLED` | Enable the Kubernetes events ingest API. | `false` | `true` |

## Tenancy defaults

| Variable | Purpose | Default | Example |
| --- | --- | --- | --- |
| `POD_SECURITY_LEVEL` | Pod Security Admission enforce level. | `baseline` | `restricted` |
| `POD_SECURITY_WARN_LEVEL` | Pod Security warning level. | `baseline` | `restricted` |
| `POD_SECURITY_AUDIT_LEVEL` | Pod Security audit level. | `baseline` | `privileged` |
| `PROJECTS_IN_USER_NAMESPACE` | Deploy projects into the user namespace. | `true` | `false` |
| `CLUSTER_DEFAULT_ENVIRONMENT` | Fallback value when registering clusters. | _(empty)_ | `staging` |
| `CLUSTER_DEFAULT_REGION` | Fallback region for new clusters. | _(empty)_ | `eu-west-1` |

## Namespace labels

| Variable | Purpose | Default | Example |
| --- | --- | --- | --- |
| `DNS_NS_LABEL_KEY` | Namespace label key for DNS automation. | `kubernetes.io/metadata.name` | `team` |
| `DNS_NS_LABEL_VALUE` | Namespace label value for DNS automation. | `kube-system` | `kubeop-shared` |
| `DNS_POD_LABEL_KEY` | Pod label key for DNS automation. | `k8s-app` | `app` |
| `DNS_POD_LABEL_VALUE` | Pod label value for DNS automation. | `kube-dns` | `coredns` |
| `INGRESS_NS_LABEL_KEY` | Namespace label key applied to ingress namespaces. | `kubeop.io/ingress` | `networking.kubernetes.io/enabled` |
| `INGRESS_NS_LABEL_VALUE` | Namespace label value applied to ingress namespaces. | `true` | `shared` |

## Namespace ResourceQuota defaults

| Variable | Purpose | Default |
| --- | --- | --- |
| `KUBEOP_DEFAULT_REQUESTS_CPU` | Total CPU requests. | `2` |
| `KUBEOP_DEFAULT_LIMITS_CPU` | Total CPU limits. | `4` |
| `KUBEOP_DEFAULT_REQUESTS_MEMORY` | Total memory requests. | `4Gi` |
| `KUBEOP_DEFAULT_LIMITS_MEMORY` | Total memory limits. | `8Gi` |
| `KUBEOP_DEFAULT_REQUESTS_EPHEMERAL` | Ephemeral storage requests. | `10Gi` |
| `KUBEOP_DEFAULT_LIMITS_EPHEMERAL` | Ephemeral storage limits. | `20Gi` |
| `KUBEOP_DEFAULT_PODS` | Pod count. | `30` |
| `KUBEOP_DEFAULT_SERVICES` | Service count. | `10` |
| `KUBEOP_DEFAULT_SERVICES_LOADBALANCERS` | LoadBalancer services. | `1` |
| `KUBEOP_DEFAULT_CONFIGMAPS` | ConfigMaps. | `100` |
| `KUBEOP_DEFAULT_SECRETS` | Secrets. | `100` |
| `KUBEOP_DEFAULT_PVCS` | PersistentVolumeClaims. | `10` |
| `KUBEOP_DEFAULT_REQUESTS_STORAGE` | Persistent storage requests. | `200Gi` |
| `KUBEOP_DEFAULT_DEPLOYMENTS_APPS` | Deployments. | `20` |
| `KUBEOP_DEFAULT_REPLICASETS_APPS` | ReplicaSets. | `40` |
| `KUBEOP_DEFAULT_STATEFULSETS_APPS` | StatefulSets. | `5` |
| `KUBEOP_DEFAULT_JOBS_BATCH` | Jobs. | `20` |
| `KUBEOP_DEFAULT_CRONJOBS_BATCH` | CronJobs. | `10` |
| `KUBEOP_DEFAULT_INGRESSES_NETWORKING_K8S_IO` | Ingresses. | `10` |
| `KUBEOP_DEFAULT_SCOPES` | Quota scopes. | `NotBestEffort` |
| `KUBEOP_DEFAULT_PRIORITY_CLASSES` | Priority class list. | _(empty)_ |

## Namespace LimitRange defaults

| Variable | Purpose | Default |
| --- | --- | --- |
| `KUBEOP_DEFAULT_LR_CONTAINER_MAX_CPU` | Max CPU per container. | `2` |
| `KUBEOP_DEFAULT_LR_CONTAINER_MAX_MEMORY` | Max memory per container. | `2Gi` |
| `KUBEOP_DEFAULT_LR_CONTAINER_MIN_CPU` | Min CPU per container. | `100m` |
| `KUBEOP_DEFAULT_LR_CONTAINER_MIN_MEMORY` | Min memory per container. | `128Mi` |
| `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULT_CPU` | Default CPU limit. | `500m` |
| `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULT_MEMORY` | Default memory limit. | `512Mi` |
| `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULTREQUEST_CPU` | Default CPU request. | `300m` |
| `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULTREQUEST_MEMORY` | Default memory request. | `256Mi` |
| `KUBEOP_DEFAULT_LR_CONTAINER_MAX_EPHEMERAL` | Max ephemeral storage. | `2Gi` |
| `KUBEOP_DEFAULT_LR_CONTAINER_MIN_EPHEMERAL` | Min ephemeral storage. | `128Mi` |
| `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULT_EPHEMERAL` | Default ephemeral limit. | `512Mi` |
| `KUBEOP_DEFAULT_LR_CONTAINER_DEFAULTREQUEST_EPHEMERAL` | Default ephemeral request. | `256Mi` |
| `KUBEOP_DEFAULT_LR_EXT_MAX` | Max extended resource value. | _(empty)_ |
| `KUBEOP_DEFAULT_LR_EXT_MIN` | Min extended resource value. | _(empty)_ |
| `KUBEOP_DEFAULT_LR_EXT_DEFAULT` | Default extended resource limit. | _(empty)_ |
| `KUBEOP_DEFAULT_LR_EXT_DEFAULTREQUEST` | Default extended resource request. | _(empty)_ |

## Project LimitRange defaults

| Variable | Purpose | Default |
| --- | --- | --- |
| `PROJECT_LR_REQUEST_CPU` | Project-level CPU request. | `100m` |
| `PROJECT_LR_REQUEST_MEMORY` | Project-level memory request. | `128Mi` |
| `PROJECT_LR_LIMIT_CPU` | Project-level CPU limit. | `1` |
| `PROJECT_LR_LIMIT_MEMORY` | Project-level memory limit. | `1Gi` |

## Load balancers & ingress

| Variable | Purpose | Default | Example |
| --- | --- | --- | --- |
| `LB_DRIVER` | Load balancer driver hint. | `metallb` | `aws` |
| `LB_METALLB_POOL` | Metallb pool name. | _(empty)_ | `production-pool` |
| `MAX_LOADBALANCERS_PER_PROJECT` | Guardrail for LoadBalancer services per project. | `1` | `3` |
| `PAAS_DOMAIN` | Base domain for generated hostnames. | _(empty)_ | `apps.example.com` |
| `PAAS_WILDCARD_ENABLED` | Allow wildcard DNS entries. | `false` | `true` |
| `ENABLE_CERT_MANAGER` | Enable Cert-Manager integration. | `false` | `true` |

## DNS providers

| Variable | Purpose | Default | Example |
| --- | --- | --- | --- |
| `DNS_PROVIDER` | DNS backend (`cloudflare`, `powerdns`, `noop`). | _(empty)_ | `cloudflare` |
| `DNS_API_URL` | Base API URL. | _(empty)_ | `https://api.cloudflare.com/client/v4` |
| `DNS_API_KEY` | Shared API key (legacy). | _(empty)_ | `apikey123` |
| `DNS_RECORD_TTL` | Record TTL in seconds. | `300` | `120` |
| `CLOUDFLARE_API_TOKEN` | Cloudflare API token. | Falls back to `DNS_API_KEY`. | `cf-secret` |
| `CLOUDFLARE_ZONE_ID` | Cloudflare zone identifier. | _(empty)_ | `abcd1234` |
| `CLOUDFLARE_API_BASE` | Override Cloudflare API base URL. | `https://api.cloudflare.com/client/v4` | `https://api.eu.cloudflare.com` |
| `PDNS_API_URL` | PowerDNS API URL. | Inherits `DNS_API_URL`. | `https://powerdns.example.com` |
| `PDNS_API_KEY` | PowerDNS API key. | Inherits `DNS_API_KEY`. | `pdns-secret` |
| `PDNS_SERVER_ID` | PowerDNS server identifier. | `localhost` | `production` |
| `PDNS_ZONE` | PowerDNS zone. | `PAAS_DOMAIN` | `apps.example.com` |

## Operator automation

| Variable | Purpose | Default | Example |
| --- | --- | --- | --- |
| `OPERATOR_NAMESPACE` | Namespace where `kubeop-operator` is deployed. | `kubeop-system` | `platform-system` |
| `OPERATOR_DEPLOYMENT_NAME` | Operator Deployment name. | `kubeop-operator` | `kubeop-operator` |
| `OPERATOR_SERVICE_ACCOUNT` | ServiceAccount used by the operator. | `kubeop-operator` | `kubeop-operator` |
| `OPERATOR_IMAGE` | Operator image reference. | `ghcr.io/vaheed/kubeop-operator-manager:latest` | `ghcr.io/vaheed/kubeop-operator-manager:v0.11.4` |
| `OPERATOR_IMAGE_PULL_POLICY` | Kubernetes pull policy. | `IfNotPresent` | `Always` |
| `OPERATOR_LEADER_ELECTION` | Enable controller-runtime leader election. | `false` | `true` |
