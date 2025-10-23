# Environment reference

kubeOP reads configuration from an optional YAML file (`CONFIG_FILE`) and environment variables. Defaults are applied first,
values from the YAML file override defaults, and environment variables take final precedence.

## Core application

::: include docs/_snippets/env-table.md
:::
| APP_ENV | string | development | Controls logging context and defaults for other settings. |
| PORT | int | 8080 | HTTP listen port for the API server. |
| LOG_LEVEL | string | info | Log level passed to zap (`debug`, `info`, `warn`, `error`). |
| CONFIG_FILE | string | (empty) | Optional path to a YAML config file. Ignored if the file does not exist. |

## Security and authentication

::: include docs/_snippets/env-table.md
:::
| ADMIN_JWT_SECRET | string | dev-admin-secret-change-me | HMAC secret used to sign administrator JWTs. Required unless `DISABLE_AUTH=true`. |
| DISABLE_AUTH | bool | false | Skip JWT validation entirely. Only use for trusted local testing. |
| KCFG_ENCRYPTION_KEY | string | dev-not-secure-key | Passphrase used to derive the AES-GCM key for stored kubeconfigs. Must be set in production. |
| ALLOW_INSECURE_HTTP | bool | false | Permit `http://` URLs in `KUBEOP_BASE_URL`. Without this, only HTTPS endpoints are accepted. |
| KUBEOP_BASE_URL | string | (empty) | External URL for links in API responses. Must be HTTPS unless `ALLOW_INSECURE_HTTP=true`. |
| ALLOW_GIT_FILE_PROTOCOL | bool | false | Enable Git `file://` protocol for local testing of repository sources. |

## Database and events

::: include docs/_snippets/env-table.md
:::
| DATABASE_URL | string | postgres://postgres:postgres@localhost:5432/kubeop?sslmode=disable | PostgreSQL DSN for the control-plane database. |
| EVENTS_DB_ENABLED | bool | true | Enable the events tables in PostgreSQL. Defaults to `true` unless a config file overrides it. |
| EVENT_BRIDGE_ENABLED | bool | false | Allow ingesting external Kubernetes events through `/v1/events/ingest`. Alias for `K8S_EVENTS_BRIDGE`. |
| K8S_EVENTS_BRIDGE | bool | false | Legacy alias for `EVENT_BRIDGE_ENABLED`. Both map to the same toggle. |

## Cluster defaults and scheduler

::: include docs/_snippets/env-table.md
:::
| CLUSTER_DEFAULT_ENVIRONMENT | string | (empty) | Default `environment` metadata applied to new clusters when omitted. |
| CLUSTER_DEFAULT_REGION | string | (empty) | Default `region` metadata applied to new clusters when omitted. |
| CLUSTER_HEALTH_INTERVAL_SECONDS | int | 60 | Polling interval for the cluster health scheduler. Values ≤ 0 fall back to 60 seconds. |

## Tenancy and namespace placement

::: include docs/_snippets/env-table.md
:::
| PROJECTS_IN_USER_NAMESPACE | bool | true | When true, projects share the bootstrap user namespace. Disable for dedicated namespaces per project. |
| POD_SECURITY_LEVEL | string | baseline | Pod Security admission `enforce` level applied to tenant namespaces. |
| POD_SECURITY_WARN_LEVEL | string | baseline | Pod Security `warn` level. Defaults to the enforce level. |
| POD_SECURITY_AUDIT_LEVEL | string | baseline | Pod Security `audit` level. Defaults to the enforce level. |
| DNS_NS_LABEL_KEY | string | kubernetes.io/metadata.name | Namespace label key for DNS automation lookups. |
| DNS_NS_LABEL_VALUE | string | kube-system | Namespace label value for DNS automation lookups. |
| DNS_POD_LABEL_KEY | string | k8s-app | Pod label key used when locating DNS controller pods. |
| DNS_POD_LABEL_VALUE | string | kube-dns | Pod label value used when locating DNS controller pods. |
| INGRESS_NS_LABEL_KEY | string | kubeop.io/ingress | Namespace label key that marks allowed ingress namespaces. |
| INGRESS_NS_LABEL_VALUE | string | true | Namespace label value that marks allowed ingress namespaces. |

## Namespace quotas (ResourceQuota)

These defaults apply when kubeOP provisions tenant namespaces. Values are strings compatible with Kubernetes quantity parsing.

::: include docs/_snippets/env-table.md
:::
| KUBEOP_DEFAULT_REQUESTS_CPU | string | 2 | CPU requests quota. |
| KUBEOP_DEFAULT_LIMITS_CPU | string | 4 | CPU limits quota. |
| KUBEOP_DEFAULT_REQUESTS_MEMORY | string | 4Gi | Memory requests quota. |
| KUBEOP_DEFAULT_LIMITS_MEMORY | string | 8Gi | Memory limits quota. |
| KUBEOP_DEFAULT_REQUESTS_EPHEMERAL | string | 10Gi | Ephemeral storage requests quota. |
| KUBEOP_DEFAULT_LIMITS_EPHEMERAL | string | 20Gi | Ephemeral storage limits quota. |
| KUBEOP_DEFAULT_PODS | string | 30 | Maximum pods per namespace. |
| KUBEOP_DEFAULT_SERVICES | string | 10 | Maximum services per namespace. |
| KUBEOP_DEFAULT_SERVICES_LOADBALANCERS | string | 1 | Maximum LoadBalancer services. |
| KUBEOP_DEFAULT_CONFIGMAPS | string | 100 | Maximum ConfigMaps. |
| KUBEOP_DEFAULT_SECRETS | string | 100 | Maximum Secrets. |
| KUBEOP_DEFAULT_PVCS | string | 10 | Maximum PersistentVolumeClaims. |
| KUBEOP_DEFAULT_REQUESTS_STORAGE | string | 200Gi | Total storage requested across PVCs. |
| KUBEOP_DEFAULT_DEPLOYMENTS_APPS | string | 20 | Maximum Deployments. |
| KUBEOP_DEFAULT_REPLICASETS_APPS | string | 40 | Maximum ReplicaSets. |
| KUBEOP_DEFAULT_STATEFULSETS_APPS | string | 5 | Maximum StatefulSets. |
| KUBEOP_DEFAULT_JOBS_BATCH | string | 20 | Maximum batch Jobs. |
| KUBEOP_DEFAULT_CRONJOBS_BATCH | string | 10 | Maximum CronJobs. |
| KUBEOP_DEFAULT_INGRESSES_NETWORKING_K8S_IO | string | 10 | Maximum Ingress resources. |
| KUBEOP_DEFAULT_SCOPES | string | NotBestEffort | ResourceQuota scopes applied to tenant namespaces. |
| KUBEOP_DEFAULT_PRIORITY_CLASSES | string | (empty) | Allowed priority classes. Leave blank to allow all. |

## Namespace limit ranges (LimitRange)

Defaults applied to container-level and extended resources.

::: include docs/_snippets/env-table.md
:::
| KUBEOP_DEFAULT_LR_CONTAINER_MAX_CPU | string | 2 | Maximum CPU per container. |
| KUBEOP_DEFAULT_LR_CONTAINER_MAX_MEMORY | string | 2Gi | Maximum memory per container. |
| KUBEOP_DEFAULT_LR_CONTAINER_MIN_CPU | string | 100m | Minimum CPU per container. |
| KUBEOP_DEFAULT_LR_CONTAINER_MIN_MEMORY | string | 128Mi | Minimum memory per container. |
| KUBEOP_DEFAULT_LR_CONTAINER_DEFAULT_CPU | string | 500m | Default CPU limit. |
| KUBEOP_DEFAULT_LR_CONTAINER_DEFAULT_MEMORY | string | 512Mi | Default memory limit. |
| KUBEOP_DEFAULT_LR_CONTAINER_DEFAULTREQUEST_CPU | string | 300m | Default CPU request. |
| KUBEOP_DEFAULT_LR_CONTAINER_DEFAULTREQUEST_MEMORY | string | 256Mi | Default memory request. |
| KUBEOP_DEFAULT_LR_CONTAINER_MAX_EPHEMERAL | string | 2Gi | Maximum ephemeral storage per container. |
| KUBEOP_DEFAULT_LR_CONTAINER_MIN_EPHEMERAL | string | 128Mi | Minimum ephemeral storage per container. |
| KUBEOP_DEFAULT_LR_CONTAINER_DEFAULT_EPHEMERAL | string | 512Mi | Default ephemeral storage limit. |
| KUBEOP_DEFAULT_LR_CONTAINER_DEFAULTREQUEST_EPHEMERAL | string | 256Mi | Default ephemeral storage request. |
| KUBEOP_DEFAULT_LR_EXT_MAX | string | (empty) | Maximum extended resource quantity (for example, GPUs). |
| KUBEOP_DEFAULT_LR_EXT_MIN | string | (empty) | Minimum extended resource quantity. |
| KUBEOP_DEFAULT_LR_EXT_DEFAULT | string | (empty) | Default extended resource limit. |
| KUBEOP_DEFAULT_LR_EXT_DEFAULTREQUEST | string | (empty) | Default extended resource request. |

## Project-level defaults

::: include docs/_snippets/env-table.md
:::
| PROJECT_LR_REQUEST_CPU | string | 100m | CPU request applied to project-scoped LimitRange. |
| PROJECT_LR_REQUEST_MEMORY | string | 128Mi | Memory request applied to project-scoped LimitRange. |
| PROJECT_LR_LIMIT_CPU | string | 1 | CPU limit applied to project-scoped LimitRange. |
| PROJECT_LR_LIMIT_MEMORY | string | 1Gi | Memory limit applied to project-scoped LimitRange. |

## Application delivery controls

::: include docs/_snippets/env-table.md
:::
| LB_DRIVER | string | metallb | Load balancer provisioning backend (`metallb` or `none`). |
| LB_METALLB_POOL | string | (empty) | Specific MetalLB pool to allocate from when `LB_DRIVER=metallb`. |
| MAX_LOADBALANCERS_PER_PROJECT | int | 1 | Hard limit on LoadBalancer services per project. |
| PAAS_DOMAIN | string | (empty) | Base domain used when generating ingress hosts. |
| PAAS_WILDCARD_ENABLED | bool | false | Allow wildcard certificates for generated domains. |
| ENABLE_CERT_MANAGER | bool | false | Assume cert-manager is installed and issue certificates automatically. |

## Webhooks and automation

::: include docs/_snippets/env-table.md
:::
| GIT_WEBHOOK_SECRET | string | (empty) | Shared secret for validating Git webhooks received at `/v1/webhooks/git`. |

## DNS providers

::: include docs/_snippets/env-table.md
:::
| DNS_PROVIDER | string | (empty) | DNS backend (`cloudflare`, `powerdns`, or custom). Lower-cased automatically. |
| DNS_API_URL | string | (empty) | Base URL for DNS API requests. |
| DNS_API_KEY | string | (empty) | Generic API key used when provider-specific tokens are not supplied. |
| DNS_RECORD_TTL | int | 300 | Default TTL (seconds) for created DNS records. |
| CLOUDFLARE_API_TOKEN | string | (empty) | Cloudflare API token. Falls back to `DNS_API_KEY` when empty. |
| CLOUDFLARE_ZONE_ID | string | (empty) | Cloudflare zone identifier. |
| CLOUDFLARE_API_BASE | string | https://api.cloudflare.com/client/v4 | Cloudflare API base URL. Defaults to `DNS_API_URL` when set. |
| PDNS_API_URL | string | (empty) | PowerDNS API endpoint. Defaults to `DNS_API_URL` when set. |
| PDNS_API_KEY | string | (empty) | PowerDNS API key. Defaults to `DNS_API_KEY` when set. |
| PDNS_SERVER_ID | string | localhost | PowerDNS server identifier. |
| PDNS_ZONE | string | (empty) | PowerDNS hosted zone. Defaults to `PAAS_DOMAIN` when set. |

## Operator automation

These values configure how kubeOP ensures the `kubeop-operator` deployment is installed in managed clusters.

::: include docs/_snippets/env-table.md
:::
| OPERATOR_NAMESPACE | string | kubeop-system | Namespace that should contain the operator deployment. |
| OPERATOR_DEPLOYMENT_NAME | string | kubeop-operator | Deployment name to validate/install. |
| OPERATOR_SERVICE_ACCOUNT | string | kubeop-operator | ServiceAccount name associated with the operator. |
| OPERATOR_IMAGE | string | ghcr.io/vaheed/kubeop-operator-manager:latest | Container image for the operator manager. |
| OPERATOR_IMAGE_PULL_POLICY | string | IfNotPresent | Kubernetes pull policy used when ensuring the operator deployment. |
| OPERATOR_LEADER_ELECTION | bool | false | Enable controller-runtime leader election in the operator deployment. |

## Derived helpers

| Helper | Description |
| --- | --- |
| AUTHORIZATION header | Use `Authorization: Bearer <token>` with a JWT signed by `ADMIN_JWT_SECRET`. |
| Maintenance mode | When enabled via API, mutating operations return HTTP 503 with the configured message. |

Consult [docs/SECURITY.md](SECURITY.md) and [docs/OPERATIONS.md](OPERATIONS.md) for guidance on rotating secrets and handling
incident response.
