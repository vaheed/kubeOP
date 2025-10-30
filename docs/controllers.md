# Controllers

Controller behavior comes from [`internal/operator/controllers/controllers.go`](https://github.com/vaheed/kubeOP/blob/main/internal/operator/controllers/controllers.go).

## TenantReconciler

- Watches: `paas.kubeop.io/v1alpha1` Tenants (`controller.NewControllerManagedBy(...).For(&v1alpha1.Tenant{})`).
- Concurrency: `MaxConcurrentReconciles=1`.
- Sequence:
  1. Fetches the Tenant (`r.Get`). If missing, reconciliation exits (`client.IgnoreNotFound`).
  2. Calls `setCondition` to ensure a `Ready` condition with type `Bootstrapped` (`Ready=True`).
  3. Sets `status.ready=true` and updates status via `r.Status().Update`.
- Delete handling: no finalizers; resources simply disappear.
- Events: none emitted (no event recorder used).
- Backoff: relies on controller-runtime defaults; no explicit requeue.

## ProjectReconciler

- Watches: Projects, owns Namespaces (`Owns(&corev1.Namespace{})`).
- Concurrency: `MaxConcurrentReconciles=1`.
- Sequence:
  1. Load Project; exit if not found.
  2. Compute namespace name `kubeop-<tenantRef>-<name>`.
  3. Create namespace with labels `app.kubeop.io/tenant` and `app.kubeop.io/project` when absent.
  4. Ensure baseline resources:
     - LimitRange `kubeop-defaults` with default/request CPU & memory (`ensureLimitRange`).
     - ResourceQuota `kubeop-quota` limiting pods and CPU/memory requests (`ensureResourceQuota`).
     - NetworkPolicy `kubeop-egress` allowing unrestricted egress (`ensureEgressPolicy`).
  5. Update status namespace, Ready=True, and set `Bootstrapped` condition.
- Error handling: if any create/update fails, errors propagate and Ready is set to False with `CreateFailed` reason.
- Events: none.
- Backoff: relies on controller-runtime defaults.

## AppReconciler

- Watches: Apps.
- Concurrency: `MaxConcurrentReconciles=2`.
- Sequence:
  1. Load App; exit if not found.
  2. Optional CPU spin when `KUBEOP_RECONCILE_SPIN_MS` is set (`os.Getenv` + busy loop) for load testing.
  3. If `spec.type == "Image"` and `spec.image` is set:
     - Manage Deployment `app-<name>` with labels `app.kubeop.io/app`.
     - Create the Deployment when absent, defaulting to 1 replica, container name `app`, port 80.
     - On update, ensure first container image matches `spec.image`.
  4. Stamp `status.revision` if empty (UTC timestamp `YYYYMMDD-HHMMSS`).
  5. Determine readiness: when the Deployment has `<1` available replicas, set Ready=False with reason `Progressing` and requeue after 5s. Otherwise Ready=True with reason `Converged`.
  6. Persist status.
- Delete handling: Deployments remain unless Kubernetes garbage-collects owner references (the controller does not set owner refs, so manual cleanup may be required).
- Events: none.

## DNSRecordReconciler

- Watches: DNSRecords.
- Sequence:
  1. Load DNSRecord; exit if not found.
  2. If `Endpoint` (from `DNS_MOCK_URL`) is set, POST `/v1/dnsrecords` with empty body.
  3. Set `status.ready=true`, `status.message="mocked"`, update status.
- No requeue logic; idempotent.

## CertificateReconciler

- Watches: Certificates; also declares ownership of Deployments, though no Deployments are currently created.
- Sequence similar to DNSRecordReconciler but hitting `/v1/certificates` and writing `status.message="issued"`.

## Shared helper: setCondition

The private `setCondition` replaces or appends a condition entry keyed by type, updating `lastTransitionTime` to `time.Now()` in UTC. All controllers depend on this helper to keep `Ready` condition histories consistent.
