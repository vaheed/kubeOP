# Roadmap

The v0.14.0 reset removed legacy compatibility layers and deprecated labels. The next milestones focus on deepening core workflows without reintroducing shims.

## Near term (0–6 weeks)

### Release inspection API
- Expose `/v1/projects/{id}/apps/{appId}/releases/{releaseId}` with diff helpers.
- Persist delivery digests and SBOM metadata that already exists in `internal/store`.
- Update [`docs/API.md`](API.md) with sample `curl` flows.

### Operator reconciliation parity
- Extend `kubeop-operator` to reconcile Services and Ingresses using the `kubeop.app.id` label.
- Mirror status fields exposed by `CollectAppStatus` so API consumers see consistent readiness data.
- Add controller and integration tests covering multi-port services and ingress annotations.

### Event retention
- Add retention windows for project events with metrics describing purge behaviour.
- Document tunables in [`docs/ENVIRONMENT.md`](ENVIRONMENT.md).
- Ensure `go test ./...` covers migration ordering and retention guards.

## Mid term (6–12 weeks)

### Credential rotation tooling
- Provide API helpers and CLI scripts for rolling admin JWT secrets without downtime.
- Harden validation to reject known development defaults outside Compose environments.

### Template versioning
- Introduce versioned application templates with diff-friendly metadata.
- Surface template history via the REST API and align docs accordingly.

## Long term (12+ weeks)

### Multi-region control plane
- Support regional API replicas with shared PostgreSQL and read-only maintenance mode.
- Document deployment patterns for highly available installations.

Progress is tracked in GitHub issues with the `roadmap` label. Each initiative should ship with updated tests, documentation, and CI coverage.
