# Versioning & compatibility

kubeOP follows Semantic Versioning and publishes compatibility metadata so automation can validate supported combinations before executing destructive actions. The `/v1/version` endpoint surfaces the same struct consumed by the CLI and logging system.

See the [API contract & release policy](./api-contract.md) for the broader
deprecation workflow and release cadence commitments.

## Build metadata

```json
{
  "version": "0.11.3",
  "commit": "<git-sha>",
  "date": "2025-10-29T10:00:00Z",
  "compatibility": {
    "minClientVersion": "0.8.16",
    "minApiVersion": "v1",
    "maxApiVersion": "v1"
  }
}
```

- **`version`** – Semantic Version string baked into the binary via `-ldflags`.
- **`commit`** / **`date`** – Git SHA and build timestamp for traceability.
- **`compatibility.minClientVersion`** – Minimum kubeOP CLI/automation release expected by this server. CI/CD pipelines should exit when their client version is older than this value.
- **`compatibility.minApiVersion` / `maxApiVersion`** – REST API range (`/v1` today). Future releases will add `/v1beta1` or `/v2` ranges here as the contract evolves.
- **`deprecation.deadline`** *(optional)* – RFC3339 timestamp indicating when the build becomes unsupported. Once the deadline is reached, kubeOP logs a warning on startup and whenever `/v1/version` is called.

## Release cadence

- Patch releases focus on bug fixes and compatibility metadata updates.
- Minor releases unlock new APIs or behaviours and will update the compatibility matrix accordingly.
- The changelog under [`docs/changelog.md`](../changelog.md) records every change; upgrade guidance references compatibility metadata when deprecated ranges are removed.

## Operational guidance

1. Automations should call `/v1/version` before issuing mutations. Abort when the reported `minClientVersion` exceeds the client's SemVer.
2. Monitor logs for `running deprecated kubeOP build` warnings. They indicate the deployment is past its supported window.
3. When preparing a release, set `-X kubeop/internal/version.rawDeprecationDeadline=<timestamp>` (and optionally `rawDeprecationNote`) to broadcast upgrade deadlines.
4. Keep the OpenAPI document (`docs/openapi.yaml`) in sync with the version metadata so API documentation reflects the current release.
