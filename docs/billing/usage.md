# Usage aggregation and billing snapshots

The kubeOP operator ships with a dedicated **Usage Writer** controller that
collects resource metrics every five minutes, projects them into hourly billing
windows, and publishes the results both on the Tenant/Project status surfaces
and in the `BillingUsage` custom resource. Platform teams gain a consistent view
of live consumption while billing pipelines can pick up immutable hourly
snapshots.

## Controller workflow

1. The controller wakes up on a five-minute cadence and truncates the current
   time to the start of the hour (for example, `2026-02-20T10`).
2. Metrics are fetched from the in-cluster metrics-server. Each Pod metric is
   attributed using the `paas.kubeop.io/{tenant,project,app}` labels so the
   controller can group usage by tenant, project namespace, and application.
3. CPU, memory, storage, egress, and load balancer hour counters are aggregated
   for every `{tenant, project, app}` tuple. When the metrics backend omits a
   value the controller writes a zero so downstream consumers always see all
   meter keys.
4. Results are written to:
   - `Tenant.status.usage`
   - `Project.status.usage`
   - `BillingUsage` resources with a stable name derived from the subject and
     billing window (`tenant-acme-2026-02-20t10`, `project-acme-dev-payments-…`)
5. `Reconciling`, `Ready`, and `Degraded` conditions are updated with
   contextual messages so `kubectl get` surfaces both health and the current
   snapshot timestamp.

Re-running the reconciliation loop for the same hour results in idempotent
updates. The controller rewrites the `BillingUsage` spec in-place and updates
status conditions without creating duplicate records.

## Live visibility with printer columns

The `Tenant` and `Project` CRDs now expose printer columns for the aggregated
meters. A quick `kubectl get` surfaces both readiness and the latest usage
snapshot:

```bash
kubectl get tenants -o wide
```

Sample output:

```
NAME   DISPLAY     READY   CPU   MEM    STORAGE   EGRESS   LBH   AGE
acme   Acme Corp   True    250m  512Mi  10Gi      1Gi      2     3h
```

Projects expose the same columns when listing namespaced resources:

```
kubectl get projects -A -o wide
NAMESPACE   NAME       TENANT   READY   CPU   MEM    STORAGE   EGRESS   LBH   AGE
acme        payments   acme     True    250m  512Mi  10Gi      1Gi      2     3h
```

## BillingUsage resources

Each billing subject (tenant, project, app) receives an hourly
`BillingUsage` record keyed by the subject reference and the window start
hour. The spec captures all meters as strings to keep the resource immutable
and API-friendly:

```yaml
apiVersion: paas.kubeop.io/v1alpha1
kind: BillingUsage
metadata:
  name: tenant-acme-2026-02-20t10
spec:
  subjectType: tenant
  subjectRef: acme
  window: 2026-02-20T10
  meters:
    cpu: 250m
    memory: 512Mi
    storage: 10Gi
    egress: 1Gi
    lbHours: "2"
status:
  conditions:
    - type: Ready
      status: "True"
      reason: UsageAggregationComplete
      message: Usage window 2026-02-20T10:00:00Z updated
```

Consumers can watch `BillingUsage` objects or stream events to feed hourly
billing pipelines without scraping operator logs.

## Troubleshooting

- **No metrics reported:** ensure the cluster has metrics-server installed and
  that namespaces carrying kubeOP workloads propagate the
  `paas.kubeop.io/{tenant,project,app}` labels to Pods. When the metrics API is
  unavailable the controller marks Tenant and Project resources as `Degraded`
  with an explanatory message.
- **Missing meters:** storage, egress, and load balancer hour counters depend on
  the backing metrics platform exposing those values. When the source omits a
  meter the controller records `0` for the hour so dashboards remain stable.
- **Custom metrics sources:** the metrics provider is pluggable through the
  `metrics.Provider` interface. Additional collectors (for example Prometheus
  queries) can be wired into the manager without changing the reconciliation
  logic.

Refer to [docs/CRDs.md](../CRDs.md) for the full schema of `Tenant`,
`Project`, and `BillingUsage` resources.
