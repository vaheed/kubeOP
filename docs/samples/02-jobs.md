# Jobs & CronJobs sample

This sample delivers Kubernetes batch workloads without additional scripting. The manifests in `samples/jobs/` inherit the kubeOP tenancy labelling scheme so they surface in project event timelines even when orchestrated manually.

## Files

- [`samples/jobs/simple-job.yaml`](../../samples/jobs/simple-job.yaml) — one-off data processing job with resource requests, deterministic container command, and a `ttlSecondsAfterFinished` guard so completed pods are cleaned up automatically.
- [`samples/jobs/cron-job.yaml`](../../samples/jobs/cron-job.yaml) — scheduled reporting CronJob that restricts concurrency, preserves the last successful runs, and publishes output to object storage.

## Usage

1. Replace the placeholder IDs and names in the `metadata.labels` block with your kubeOP cluster, project, and app identifiers.
2. Apply the manifest with `kubectl` against the tenant namespace:
   ```bash
   kubectl apply -f samples/jobs/simple-job.yaml
   kubectl get jobs -n <tenant-namespace>
   ```
3. Inspect job logs and events using the kubeOP API:
   ```bash
   curl -s $AUTH_H "http://localhost:8080/v1/projects/<project-id>/apps/<app-id>/events" | jq
   ```
4. Delete the CronJob when experimenting to avoid stray schedules:
   ```bash
   kubectl delete -f samples/jobs/cron-job.yaml
   ```

The manifests intentionally keep container images and arguments simple so you can fork them for your batch workloads or template catalog entries. Update environment variables and resources according to your workload profile.
