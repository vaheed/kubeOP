# Auditing app releases

kubeOP records an immutable release every time an application deployment
succeeds. Releases capture the deployment spec digest, rendered object
summaries, Helm inputs, load balancer usage, and warnings so operators can audit
rollouts and compare revisions.

## Prerequisites

- An application deployed through `/v1/projects/{id}/apps` or template deploy.
- An admin or project token (`Authorization: Bearer ...`).

## Fetch the latest releases

List the newest releases for an app (default 20 entries) and inspect the
metadata that kubeOP persisted after deployment.

```bash
curl -s $AUTH_H \
  "http://localhost:8080/v1/projects/<project-id>/apps/<app-id>/releases?limit=5" | jq
```

Response fields:

- `specDigest` / `renderDigest` – SHA-256 hashes of the sanitized spec and
  rendered manifest summary.
- `spec.source` – `image`, `manifests`, or `helm` including the exact inputs.
- `renderedObjects[]` – kind/name pairs detected during planning.
- `loadBalancers` – requested versus existing counts at deploy time.
- `warnings[]` – any non-blocking issues detected during validation.

## Paginate through history

Use the `nextCursor` value to fetch older releases. Each page is ordered from
newest to oldest, making it easy to walk backwards through time.

```bash
NEXT=$(curl -s $AUTH_H \
  "http://localhost:8080/v1/projects/<project-id>/apps/<app-id>/releases?limit=10" | jq -r '.nextCursor')

curl -s $AUTH_H \
  "http://localhost:8080/v1/projects/<project-id>/apps/<app-id>/releases?limit=10&cursor=${NEXT}" | jq
```

When `nextCursor` is empty, you have reached the oldest known release.

## Compare releases

Combine the release API with `jq` to diff spec hashes or rendered object lists.

```bash
curl -s $AUTH_H \
  "http://localhost:8080/v1/projects/<project-id>/apps/<app-id>/releases?limit=2" \
  | jq '.["releases"] | map({id, specDigest, renderDigest, helmChart: .helmChart})'
```

Use the digests to confirm whether two deploys were identical or if Helm values,
manifests, or image inputs changed.

For a copy-paste walkthrough that deploys an app and inspects the
resulting releases, see [`../TUTORIALS/release-audit.md`](../TUTORIALS/release-audit.md).
