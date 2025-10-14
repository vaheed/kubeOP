# kubeOP Watcher Bridge

The watcher bridge is a companion binary that mirrors Kubernetes resource
changes into kubeOP’s `/v1/events/ingest` endpoint. It runs out of
cluster (or as a privileged in-cluster deployment) using a kubeconfig
supplied by kubeOP during cluster registration.

## Capabilities

- Watches Pods, Deployments, Services, Ingresses, Jobs, CronJobs,
  HorizontalPodAutoscalers, PersistentVolumeClaims, ConfigMaps, Secrets,
  core/v1 Events, and cert-manager Certificates with shared informers.
- Applies a label selector (default
  `kubeop.project.id,kubeop.app.id,kubeop.tenant.id`) to reduce traffic to
  tenant-labelled objects only.
- Persists per-kind resource versions in BoltDB so restarts resume from
  the last bookmark.
- Streams batches (≤200 events or 1 second) to kubeOP, compressing
  payloads over 8 KiB and retrying with exponential backoff.
- Provides `/healthz`, `/readyz`, and `/metrics` (Prometheus) endpoints
  on the configured listen address (default `:8081`).

## Environment variables

| Variable | Default | Description |
| --- | --- | --- |
| `CLUSTER_ID` | _required_ | UUID of the kubeOP cluster; included in every event payload. |
| `KUBEOP_EVENTS_URL` | _required_ | HTTPS endpoint for `/v1/events/ingest`. |
| `KUBEOP_TOKEN` | _required_ | Bearer token accepted by the kubeOP API. |
| `KUBECONFIG` | _optional_ | Path to the kubeconfig used to access the target cluster. If unset, in-cluster credentials are used. |
| `WATCH_KINDS` | all supported | Comma-separated list of kinds to watch (e.g. `Pods,Deployments`). Case-insensitive. |
| `LABEL_SELECTOR` | `kubeop.project.id,kubeop.app.id,kubeop.tenant.id` | Selector applied to every informer. Pure existence keys feed into the watcher’s label guard. |
| `BATCH_MAX` | `200` | Maximum events per POST. Values above 200 are clamped. |
| `BATCH_WINDOW_MS` | `1000` | Max time in milliseconds before flushing a batch. |
| `HTTP_TIMEOUT_SECONDS` | `15` | Client timeout when calling kubeOP. |
| `STORE_PATH` | `/var/lib/kubeop-watcher/state.db` | Location of the BoltDB checkpoint file. Ensure the directory is writable and persisted. |
| `HEARTBEAT_MINUTES` | `5` | When >0, emit a synthetic “Watcher” event at the interval to signal liveness. |
| `WATCHER_LISTEN_ADDR` | `:8081` | Address for the probe/metrics HTTP server. |
| `LOG_LEVEL` | `info` | See [`internal/logging`](../internal/logging). |

All other logging-related environment variables (`LOGS_ROOT`, `LOG_DIR`,
etc.) follow the API server behaviour.

## Deployment outline

1. **Grant read permissions** – The watcher needs `get`, `list`, and
   `watch` on the supported resources. Example ClusterRole:

   ```yaml
   apiVersion: rbac.authorization.k8s.io/v1
   kind: ClusterRole
   metadata:
     name: kubeop-watcher
   rules:
     - apiGroups: [""]
       resources: [pods, services, configmaps, secrets, events, persistentvolumeclaims]
       verbs: [get, list, watch]
     - apiGroups: ["apps"]
       resources: [deployments]
       verbs: [get, list, watch]
     - apiGroups: ["batch"]
       resources: [jobs, cronjobs]
       verbs: [get, list, watch]
     - apiGroups: ["autoscaling"]
       resources: [horizontalpodautoscalers]
       verbs: [get, list, watch]
     - apiGroups: ["networking.k8s.io"]
       resources: [ingresses]
       verbs: [get, list, watch]
     - apiGroups: ["cert-manager.io"]
       resources: [certificates]
       verbs: [get, list, watch]
   ```

2. **Bind to a ServiceAccount** – Create a ServiceAccount (for
   in-cluster deployments) and `ClusterRoleBinding` pointing at the role
   above.

3. **Provide storage** – Mount a persistent volume to `/var/lib/kubeop-watcher`
   so restarts reuse the BoltDB checkpoint.

4. **Supply kubeconfig** – Either mount a kubeconfig file through a
   Secret and set `KUBECONFIG`, or run the watcher out of cluster with a
   standard kubeconfig path on disk.

5. **Configure environment** – Set `CLUSTER_ID`, `KUBEOP_EVENTS_URL`,
   and `KUBEOP_TOKEN`. The token should be scoped to the ingest endpoint
   and stored outside the container image (e.g., Kubernetes Secret or CI
   secret store).

Example Deployment fragment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: kubeop-watcher
  namespace: kube-system
spec:
  replicas: 1
  selector:
    matchLabels:
      app: kubeop-watcher
  template:
    metadata:
      labels:
        app: kubeop-watcher
    spec:
      serviceAccountName: kubeop-watcher
      containers:
        - name: watcher
        image: ghcr.io/vaheed/kubeop:watcher
          env:
            - name: CLUSTER_ID
              value: "<cluster-uuid>"
            - name: KUBEOP_EVENTS_URL
              value: "https://kubeop.example.com/v1/events/ingest"
            - name: KUBEOP_TOKEN
              valueFrom:
                secretKeyRef:
                  name: kubeop-watcher
                  key: token
            - name: KUBECONFIG
              value: /kube/config
          volumeMounts:
            - name: kubeconfig
              mountPath: /kube
              readOnly: true
            - name: state
              mountPath: /var/lib/kubeop-watcher
      volumes:
        - name: kubeconfig
          secret:
            secretName: kubeop-watcher-kubeconfig
        - name: state
          persistentVolumeClaim:
            claimName: kubeop-watcher
```

## Metrics

The watcher exports Prometheus metrics under `/metrics` including:

- `kubeop_watcher_events_total{kind,event_type}` – events accepted for delivery.
- `kubeop_watcher_events_dropped_total{reason}` – filtered or deduplicated events.
- `kubeop_watcher_batches_total{result}` – batch send results (`success`/`failure`).
- `kubeop_watcher_queue_depth` – in-flight queue length.
- `kubeop_watcher_last_successful_push_timestamp` – Unix timestamp of the last 2xx response from kubeOP.

Integrate these with kubeOP’s dashboards/alerting to flag stale bridges
or ingestion failures.
