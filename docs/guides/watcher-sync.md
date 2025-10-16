# Watcher sync pipeline

The optional kubeOP watcher streams Kubernetes resource changes back to the control plane using a deduplicating sink. This guide covers configuration, auto-deploy, manual installation, and diagnostics.

## What the watcher observes

- Dynamic informers (see `internal/watch/kinds.go`) watch Deployments, ReplicaSets, StatefulSets, Services, Ingresses, Jobs, CronJobs, and Events.
- Events are filtered by label selector `kubeop.project.id,kubeop.app.id,kubeop.tenant.id`. Resources missing any of these labels are dropped to avoid leaking unrelated workloads.
- Each event is normalised into `sink.Event` containing cluster ID, kind, namespace/name, summary, and a deduplication key (`uid#resourceVersion`).

## Auto-deployment workflow

1. Cluster registration (`POST /v1/clusters`) checks configuration.
2. If `WATCHER_AUTO_DEPLOY=true` (or `KUBEOP_BASE_URL` is set and no override disables it), kubeOP:
   - Generates a JWT using `GenerateWatcherToken` (cluster ID claim, expiry).
   - Applies namespace, ServiceAccount, Role/RoleBinding, Secret, PVC, and Deployment via `internal/watcherdeploy`.
   - Waits for readiness when `WATCHER_WAIT_FOR_READY=true` (default).
3. Watcher pods mount the kubeconfig secret, start informers, perform a `/v1/watchers/handshake`, and send batches to `WATCHER_EVENTS_URL` once enabled.

Logs show the auto-deploy decision with `watcher_auto_deploy` fields (`enabled`, `reason`). Use `/v1/clusters` response to confirm deployment success.

## Manual deployment

Disable auto-deploy and run the watcher yourself when clusters cannot reach the control plane or require custom manifests.

1. Provide a static token by setting `WATCHER_TOKEN=<random-hex>` in the control plane environment. This bypasses per-cluster JWT minting.
2. Build the watcher binary (`go build -o kubeop-watcher ./cmd/kubeop-watcher`) or download the published image/binary for your platform.
3. Create a kubeconfig with cluster-admin permissions for the target cluster and store it securely (e.g. `watcher.kubeconfig`).
4. On the watcher host, export required variables:
   ```bash
   export CLUSTER_ID=<cluster-id>
   export KUBEOP_BASE_URL=https://kubeop.example.com
   export KUBEOP_TOKEN=<same value as WATCHER_TOKEN>
   export LABEL_SELECTOR="kubeop.project.id,kubeop.app.id,kubeop.tenant.id"
   export WATCH_KINDS=deployments.apps,replicasets.apps,ingresses.networking.k8s.io,services,events
   ```
5. Run the binary:
   ```bash
   ./kubeop-watcher --kubeconfig watcher.kubeconfig
   ```

Mount persistent storage to `STORE_PATH` (default `/var/lib/kubeop-watcher/state.db`) so informer resource versions survive restarts.

## Sink behaviour

- `internal/sink.Sink` buffers events up to `WATCHER_BATCH_MAX` (default 200) or `WATCHER_BATCH_WINDOW_MS` (default 1000 ms).
- Payloads larger than 8 KiB are gzipped before POSTing to the control plane.
- Retries use exponential backoff starting at 250 ms up to 30 s. When a persistent queue is configured, kubeOP now stores the batch locally after the first failed attempt instead of retrying indefinitely, reducing API pressure while connectivity is down. Stored batches re-enqueue automatically after the next successful handshake.
- Successful deliveries set a readiness flag so `/readyz` reports healthy.

## Health checks and metrics

- `/metrics` exposes counters/gauges for queue depth, dropped events (missing labels, duplicates, decode errors), retries, and heartbeats.
- When watchers cannot reach kubeOP, logs show `failed to deliver batch` with retry metadata. Inspect network connectivity to `WATCHER_EVENTS_URL`; batches persist locally until connectivity is restored.
- Control plane metrics expose sink delivery counters once ingest is available; until then, watchers emit warnings that delivery is disabled.

## Failure handling

- **Missing labels** – ensure workloads created outside kubeOP include `kubeop.project.id` and `kubeop.app.id`. Without them the watcher drops events.
- **Token mismatch** – when using static tokens, keep `WATCHER_TOKEN` (control plane) and `KUBEOP_TOKEN` (watcher) in sync. Rotate both sides simultaneously.
- **Namespace drift** – deleting `kubeop-system` removes watcher assets. Re-run cluster registration or redeploy using watcherdeploy manifests.
- **PVC issues** – the watcher stores informer state on a PVC. If the volume is deleted, the watcher will resync from scratch; expect an initial flood of events once ingest is active.

Keep watchers deployed—queued events are cached locally if the API is unreachable and are flushed automatically after a successful handshake.
