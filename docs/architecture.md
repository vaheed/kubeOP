# Architecture

The architecture description is sourced from the Go packages in this repository.

## Control plane components

- **Manager API** – [`cmd/manager/main.go`](https://github.com/vaheed/kubeOP/blob/main/cmd/manager/main.go#L18-L77) wires logging, configuration parsing, database connectivity, the KMS envelope, HTTP startup, and the optional usage aggregator (`KUBEOP_AGGREGATOR`). The HTTP server is provided by [`internal/api/Server`](https://github.com/vaheed/kubeOP/blob/main/internal/api/server.go#L25-L76) and exposes health checks, Prometheus metrics, and REST endpoints under `/v1/...`.
- **Operator** – [`cmd/operator/main.go`](https://github.com/vaheed/kubeOP/blob/main/cmd/operator/main.go#L19-L72) configures controller-runtime with the `paas.kubeop.io/v1alpha1` scheme, HTTP probes, metrics, and the controllers registered from [`internal/operator/controllers`](https://github.com/vaheed/kubeOP/blob/main/internal/operator/controllers/controllers.go#L29-L210). Optional mock integrations call `DNS_MOCK_URL` and `ACME_MOCK_URL` when reconciling DNSRecords and Certificates.
- **Usage aggregator** – [`internal/usage/Aggregator`](https://github.com/vaheed/kubeOP/blob/main/internal/usage/aggregator.go#L9-L33) rolls up raw usage into hourly buckets when enabled.

## Custom resources

The CRDs under [`deploy/k8s/crds`](https://github.com/vaheed/kubeOP/tree/main/deploy/k8s/crds) are implemented in Go structs in [`internal/operator/apis/paas/v1alpha1`](https://github.com/vaheed/kubeOP/blob/main/internal/operator/apis/paas/v1alpha1/types.go). They cover tenants, projects, apps, DNS records, certificates, policies, and registries.

## Reconciliation flow

1. **Tenants** – [`TenantReconciler`](https://github.com/vaheed/kubeOP/blob/main/internal/operator/controllers/controllers.go#L29-L63) loads the `Tenant`, sets the `Ready` condition to `True`, and persists status.
2. **Projects** – [`ProjectReconciler`](https://github.com/vaheed/kubeOP/blob/main/internal/operator/controllers/controllers.go#L65-L149) creates a namespaced workspace named `kubeop-<tenant>-<project>`, ensures default `LimitRange`, `ResourceQuota`, and `NetworkPolicy` objects, and updates `status.namespace` and `status.conditions`.
3. **Apps** – [`AppReconciler`](https://github.com/vaheed/kubeOP/blob/main/internal/operator/controllers/controllers.go#L151-L217) optionally spins CPU for testing (`KUBEOP_RECONCILE_SPIN_MS`), manages an `apps/v1.Deployment` for `spec.type == "Image"`, stamps `status.revision`, and sets the Ready condition once the Deployment reports an available replica.
4. **DNS records** – [`DNSRecordReconciler`](https://github.com/vaheed/kubeOP/blob/main/internal/operator/controllers/controllers.go#L219-L244) optionally POSTs to `DNS_MOCK_URL`, then marks `status.ready` and `status.message`.
5. **Certificates** – [`CertificateReconciler`](https://github.com/vaheed/kubeOP/blob/main/internal/operator/controllers/controllers.go#L246-L269) optionally POSTs to `ACME_MOCK_URL`, then marks the resource ready.

All controllers use `controller-runtime` with concurrency bounds defined in the same file. Status updates rely on the shared `setCondition` helper, ensuring every reconciliation publishes a `Ready` condition consistent with the business logic.

## Supporting services

- **HTTP facades** – [`cmd/admission`](https://github.com/vaheed/kubeOP/blob/main/cmd/admission/main.go#L13-L31), [`cmd/delivery`](https://github.com/vaheed/kubeOP/blob/main/cmd/delivery/main.go#L13-L31), and [`cmd/meter`](https://github.com/vaheed/kubeOP/blob/main/cmd/meter/main.go#L13-L31) expose identical `/healthz`, `/readyz`, `/version`, and `/metrics` endpoints on configurable addresses (defaults `:8090`, `:8091`, `:8092`). They reuse `internal/api.PromHandler` for metrics and embed `internal/version` metadata.
- **Health probe binary** – [`cmd/healthcheck`](https://github.com/vaheed/kubeOP/blob/main/cmd/healthcheck/main.go#L9-L24) exits non-zero when the configured `HEALTH_URL` does not return HTTP 200 within two seconds. The manager Dockerfile copies it as `/hc` for container health checks.
- **Mock integrations** – [`cmd/dnsmock`](https://github.com/vaheed/kubeOP/blob/main/cmd/dnsmock/main.go#L14-L25) and [`cmd/acmemock`](https://github.com/vaheed/kubeOP/blob/main/cmd/acmemock/main.go#L15-L28) run simple HTTP servers returning deterministic JSON payloads. The operator posts to them when `DNS_MOCK_URL` or `ACME_MOCK_URL` are set.
- **Webhook delivery** – [`internal/webhook/Client`](https://github.com/vaheed/kubeOP/blob/main/internal/webhook/webhook.go#L14-L61) signs payloads with `X-KubeOP-Signature` and retries failed posts.
- **Metrics** – [`internal/metrics`](https://github.com/vaheed/kubeOP/blob/main/internal/metrics/metrics.go#L5-L43) registers counters and summaries for business object creation, webhook attempts, invoice lines, and database latency. Both the manager API and operator expose `/metrics` (controller-runtime metrics server for the operator, `PromHandler` for the manager).
- **RBAC and networking** – The Helm chart renders the service account, ClusterRole, and ClusterRoleBinding defined in [`deploy/k8s/operator/rbac.yaml`](https://github.com/vaheed/kubeOP/blob/main/deploy/k8s/operator/rbac.yaml#L1-L36). Default namespace isolation is implemented through the `NetworkPolicy` created by the project reconciler and the chart template [`charts/kubeop-operator/templates/networkpolicy.yaml`](https://github.com/vaheed/kubeOP/blob/main/charts/kubeop-operator/templates/networkpolicy.yaml).
