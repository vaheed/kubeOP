# kubeOP Multi-Epoch Roadmap

This roadmap organizes kubeOP's strategic initiatives into six delivery epochs. Each epoch is self-contained and designed to deliver production-grade capabilities, while also enabling parallel execution when prerequisites are satisfied. Every deliverable concludes with tests, documentation, and operational guidance.

## Epoch I — Delivery Engine & Schema

### Goals
- Provide first-class support for all application delivery mechanisms (Helm repo, Helm OCI, Git apps, raw manifests, Kustomize, OCI bundles).
- Introduce reusable, schema-backed application templates for curated stacks.
- Persist delivery metadata and rendered revisions for auditing and rollbacks.
- Expose clear APIs and documentation for tenants, operators, and automation.

### Key Features
- **Delivery Engine**: Extend `internal/delivery` (new package) to render Helm/Kustomize/raw content, compute revisions, and apply to Kubernetes via server-side apply.
- **Delivery Types** *(Done — PR TBD)*: Support `helmRepo`, `helmOCI`, `git`, `raw`, `kustomize`, `ociBundle` with canonical kubeOP labels (`kubeop.cluster.id`, `kubeop.project.id/name`, `kubeop.app.id/name`, `kubeop.tenant.id`). `helmOCI` deliveries are ✅ (see [Helm OCI tutorial](./TUTORIALS/helm-oci-chart.md)). Git manifest/kustomize deliveries are ✅ (see [Git delivery walkthrough](./TUTORIALS/git-delivery.md)), and OCI bundles are ✅ (see [OCI bundle tutorial](./TUTORIALS/oci-bundle-delivery.md)).
- **App Templates**: Add `app_templates` table with JSON Schema validation, defaults, examples, and reusable delivery specs.
- **Credential Stores** *(Done — [Credential stores tutorial](./TUTORIALS/credential-stores.md))*: Persist Git and registry credentials with per-user/tenant scoping and encryption.
- **Event bridge ingest** *(Done — `/v1/events/ingest` with `K8S_EVENTS_BRIDGE=true`)*: Accept Kubernetes event batches from remote collectors and expose per-batch accepted/dropped summaries.
- **Releases & Audit** *(Done — PR TBD)*: Track delivery digests, rendered object hashes, status, timestamps, and logs in a new `releases` table.
- **API Endpoints**: Implement CRUD and deployment APIs using Chi (`cmd/api`), plus validation endpoints.
- **Documentation**: Comprehensive docs in `docs/apps/*.md` with runnable examples.

### Dependencies
- Go toolchain 1.24.3 (per `go.mod`).
- PostgreSQL migrations via `internal/store/migrations` (new versions required).
- Existing logging framework (`internal/logging`).
- Helm and Kustomize CLI libraries vendored via Go modules.
- Optional: Cosign/Notation libraries for signature verification.

### Milestones & Deliverables
1. **Schema & Storage**
   - New migrations for `apps.delivery`, `app_templates`, `git_credentials`, `registry_credentials`, `releases`.
   - Update `internal/store` models and repositories.
2. **Rendering Engine**
   - Implement delivery handlers with signature verification, Git checkout pipeline, and SBOM metadata capture.
   - Add unit/integration tests in `testcase/` covering rendering, validation, and pruning.
3. **Controller & Reconciler**
   - Extend reconciler to perform fetch-render-apply, server-side apply with pruning, and dry-run validation.
   - Emit structured events for fetch/render/validate/apply/prune.
4. **API Surface**
   - Add Chi handlers for templates, apps, deployments, releases, credentials, and validation.
   - Update OpenAPI spec (`docs/openapi.yaml`) and regenerate client stubs if applicable.
5. **Documentation & Examples**
   - Populate `docs/apps/` with delivery-type guides (minimal and advanced examples).
   - Provide template registration and deployment walkthroughs.

### Implementation Steps
1. Plan migrations and update `docs/changelog.md` with new schema entries.
2. Implement `Delivery` struct with discriminated union semantics in `internal/apps`.
3. Build rendering engines:
   - Helm repo + OCI loaders with signature verification toggles.
   - Git fetchers using go-git or libgit2 with commit pinning.
   - Raw/Kustomize/OCI bundle renderers with validation.
4. Update reconciler to use new delivery structs and to persist releases.
5. Implement credential CRUD endpoints with encrypted storage.
6. ✅ Add validation endpoint performing schema validation, render dry-run, and policy checks (documented in [App validation dry-run workflow](./guides/app-validation.md)).
7. Write exhaustive tests (`testcase/apps_*_test.go`).
8. Update docs and provide runnable curl/kubeopctl snippets.

### Technical Notes & Examples
- Use `valuesFrom` to merge ConfigMap/Secret values for Helm deliveries.
- Enforce JSON Schema validation using `github.com/santhosh-tekuri/jsonschema/v5` or equivalent.
- Apply Kubernetes manifests with server-side apply and force-conflicts flag for prune-safe updates.
- Example Helm repo payload:
  ```json
  {
    "type": "helmRepo",
    "helmRepo": {
      "repoUrl": "https://helm.nginx.com/stable",
      "chart": "nginx-ingress",
      "version": "1.1.0",
      "valuesYAML": "controller:\n  service:\n    type: ClusterIP\n"
    }
  }
  ```
- Example Git delivery payload:
  ```json
  {
    "type": "git",
    "git": {
      "url": "git@github.com:tenant/app.git",
      "authRef": "cred-uuid",
      "ref": "refs/heads/main",
      "path": "deploy/overlays/prod",
      "mode": "kustomize",
      "patches": [
        "patches/cpu-overlay.yaml"
      ]
    }
  }
  ```

---

## Epoch II — CRD Migration (Shadow → Cutover)

### Goals
Replace the legacy `kubeop-watcher` with a controller-based architecture where every workload, configuration, and ingress rule is managed through Custom Resource Definitions (CRDs). This delivers a single source of truth shared by the API, UI, and Kubernetes clusters while still allowing users to interact via `kubectl`.

### Strategy Overview
- Run the migration in shadow mode alongside legacy components before cutting over.
- Provide adoption tooling and documentation to assist tenants through the transition.
- Ensure observability, auditing, and operational readiness throughout the migration.

### Phase 0 — Foundation *(✅ Bootstrapped in repo)*
- Bootstrap the `kubeop-operator/` module using `controller-runtime`. *(Complete — see [`kubeop-operator/`](https://github.com/vaheed/kubeOP/tree/main/kubeop-operator) for the initial manager, CRD types, tests, and Makefile.)*
- Establish project layout:
  ```
  kubeop-operator/
  ├── api/v1alpha1/       # CRD type definitions
  ├── controllers/        # Reconcilers for each CRD
  ├── cmd/manager/        # Operator entrypoint
  ├── config/{crd,rbac,manager}/
  ├── Makefile, go.mod
  ```
- Dependencies: `sigs.k8s.io/controller-runtime`, `k8s.io/api`, `k8s.io/apimachinery`.
- Tooling: `controller-gen`, `kustomize`, `golangci-lint`.

### Phase 1 — CRD Design
- Define CRDs for `App`, `ConfigBundle`, `IngressRule`, and `SecretRef`.
- Example `App` resource:
  ```yaml
  apiVersion: kubeop.io/v1alpha1
  kind: App
  metadata:
    name: web-01
    namespace: user-alice
  spec:
    image: nginx:1.27
    replicas: 2
    ingress:
      hosts: ["web.alice.example.com"]
    env:
      - name: MODE
        value: prod
  status:
    conditions: []
  ```
- Derived resources include labels:
  ```
  kubeop.io/managed: "true"
  kubeop.io/owner-kind: App
  kubeop.io/owner-uid: <uuid>
  ```
- Retain `kubeop.io/app-id` to link CRDs with database records.

### Phase 2 — Operator Implementation
- ✅ Run a single `kubeop-operator` Deployment per cluster (v0.10.0).
- Implement reconcilers for `App`, `ConfigBundle`, and `IngressRule` resources.
- ✅ App reconciler updates the Ready condition and observed generation so status reflects
  the latest reconcile cycle (v0.8.30).
- Generate Deployment, Service, Ingress, and HPA manifests with spec hashing for idempotence.
- Track and update `status.conditions` and `status.availableReplicas` on `App` resources.
- Handle cleanup through owner references and finalizers, and add health probes, leader election, and structured logging.

### Phase 3 — Multi-Tenant Guardrails
- Ensure namespace-per-tenant isolation.
- Grant tenants CRUD access to their CRDs while denying edits to managed Kubernetes resources.
- Enforce policies (Kyverno/Gatekeeper) blocking changes to objects labeled `kubeop.io/managed=true` unless executed by the `kubeop-operator` service account.
- Optionally deploy a validating webhook for additional enforcement.

### Phase 4 — API Integration *(✅ Completed in v0.10.1)*
- API writes and deletes `App` CRDs alongside legacy resources so the operator becomes the source of truth.
- `GET /v1/projects/{id}/apps*` responses now surface CRD `resourceVersion`, `uid`, `observedGeneration`, and `conditions`.
- Mutating endpoints require `If-Match` headers and enforce Kubernetes-style `resourceVersion` concurrency checks.
- A dedicated `k8s_crds` mirror table tracks CRD identity, spec hashes, statuses, and soft deletions for auditing.

### Phase 5 — Migration (Shadow → Cutover) *(✅ Completed in v0.11.0)*
- **Shadow Mode parity checks** now diff rendered objects against live cluster state during API writes, logging drift summaries and blocking divergent rollouts until the operator reconciles the CRD-backed plan.
- **Adoption tooling** ships with documented workflows (`docs/guides/app-adoption.md`) for generating `App` CRDs from unmanaged Deployments, applying kubeOP ownership labels, and replaying delivery specs through the API.
- **Cutover enforcement** disables the legacy watcher path, routes all new workload lifecycle actions through the operator, and ensures guardrails prevent direct edits to managed Deployments, Services, Ingresses, and HPAs.

### Phase 6 — Helm & Templates Support
- Extend the `App` CRD to support Helm-backed sources:
  ```yaml
  source:
    type: helm
    repo: https://helm.nginx.com/stable
    chart: nginx-ingress
    version: 1.1.0
    values:
      controller.service.type: ClusterIP
  ```
- Render Helm charts within the operator using the Helm SDK.

### Phase 7 — Observability & Auditing
- Emit JSON logs for each reconcile cycle.
- Publish Prometheus metrics: `reconcile_duration_seconds`, `reconcile_errors_total`, and `managed_resources_total`.
- Feed CRD updates and audit webhook data into the API activity feed.

### Phase 8 — Deprecate kubeop-watcher
- Remove watcher code and manifests.
- Drop `WATCHER_*` environment variables.
- Update documentation and CI pipelines to reflect the new architecture.
- Validate control plane health after removal.

### Deliverables
- `kubeop-operator/api/v1alpha1/` — CRD definitions.
- `kubeop-operator/controllers/` — reconcilers.
- `kubeop-operator/cmd/manager/` — operator entrypoint.
- `docs/CRD-GUIDE.md` — user documentation.
- Samples library (planned) — ready-to-apply YAMLs for tutorials and smoke tests.
- API/UI in full sync with CRD state.
- Legacy watcher completely removed.

### User Experience After Migration
- Users can:
  ```bash
  kubectl get apps
  kubectl describe app web
  kubectl edit app web
  ```
- Direct edits to derived resources are blocked with guidance to edit the `App` CRD instead.

---

## Epoch III — Metering Service (kubeop-meter)

### Goals
- Launch a dedicated metering service (`kubeop-meter`) with its own deployment, database, and CI pipeline.
- Provide robust hourly usage reporting and health endpoints for billing and observability.
- Ensure the service operates independently of the core kubeOP API with optional proxying.

### Key Features
- **New Module**: Create `./meter/` Go module producing the `kubeop-meter` binary.
- **Database Isolation**: Connect to dedicated Postgres via `METER_DB_DSN`; optional read-only kubeOP DB access.
- **Collectors**: Poll clusters for usage metrics, fallback to resource requests, roll up to hourly buckets, handle backfill.
- **API**: Serve `/v1/metrics/usage` and `/v1/metrics/health` with cursor-based pagination and filtering.
- **Packaging**: Distroless Docker image (`meter/Dockerfile`), Helm chart under `helm/meter/`, GitHub Actions workflow for build/test/push.
- **Make Targets**: `make meter/build`, `meter/docker`, `meter/migrate`, `meter/test`.
- **Security & Auth**: Tenant-scoped access, secrets via env vars, encrypted credentials.

### Dependencies
- Go 1.24.3 toolchain.
- PostgreSQL migrations using `meter/migrations/` with golang-migrate.
- Kubernetes metrics APIs (metrics-server, cAdvisor); optional network sources (nfacct/cilium).
- Existing logging/metrics helpers may be vendored or re-used.
- GitHub Container Registry for publishing images.

### Milestones & Deliverables
1. **Project Bootstrap**
   - Initialize Go module in `meter/`, shared logging/util packages if needed.
   - Scaffold CLI entrypoint with configuration parsing.
2. **Database Schema & Migrations**
   - Implement `usage_hourly` table plus indexes and optional materialized views.
   - Add migration runner and integration tests against Postgres (Docker compose or test harness).
3. **Collectors & Rollups**
   - Implement cluster pollers respecting `SCAN_INTERVAL`, `BACKFILL_HOURS`, network toggles.
   - Attribute metrics using kubeOP labels; persist to DB idempotently.
4. **API Layer**
   - Expose usage and health endpoints with filtering, grouping, and pagination.
   - Add JSON schema validation and OpenAPI documentation (`docs/api/meter.yaml`).
5. **Packaging & Deployment**
   - Build minimal Docker image, Helm chart with secrets/HPA/PDB, and GitHub Actions workflow for CI/CD.
   - Provide sample Kubernetes manifests under `helm/meter/` and integration tests.
6. **Documentation**
   - Update `docs/operations.md` and new `docs/metering.md` with setup, configuration, and troubleshooting.
   - Provide curl examples demonstrating queries, grouping, and cursor usage.

### Implementation Steps
1. Create module skeleton with `cmd/kubeop-meter/main.go` handling configuration and logging.
2. Define configuration struct (env parsing) for DSNs, intervals, toggles.
3. Implement data access layer with upsert logic for `usage_hourly`.
4. Build collector framework to poll Kubernetes APIs; abstract data sources for testing.
5. Implement HTTP handlers with pagination and unit tests.
6. Write migrations and automated tests (use `docker-compose` or test containers).
7. Build Dockerfile with multi-stage build → distroless.
8. Author Helm chart and sample `values.yaml` for cluster deployment.
9. Add CI workflow to run `go test`, `go vet`, `golangci-lint` (optional), build binary, build/push image, and upload artifacts.
10. Document operational guidance and integration points with main kubeOP API (optional reverse proxy).

### Technical Notes & Examples
- Example environment configuration:
  ```bash
  export METER_DB_DSN="postgres://meter:secret@db:5432/meter?sslmode=disable"
  export KUBEOP_DB_RO_DSN="postgres://kubeop_ro:secret@db:5432/kubeop?sslmode=verify-full"
  export SCAN_INTERVAL="1m"
  export BACKFILL_HOURS="24"
  ```
- Sample usage query:
  ```bash
  curl -sS "$METER_BASE_URL/v1/metrics/usage?since=2025-01-01T00:00:00Z&until=2025-01-02T00:00:00Z&granularity=hour&resources=cpu_seconds,mem_byte_seconds&groupBy=hour,app"
  ```
- Ensure collectors gracefully handle kubeOP API downtime by caching cluster credentials.
- Use `USER 65532` in Dockerfile for distroless images.

---

## Epoch IV — Samples & Zero-to-PaaS

### Goals
- Provide a comprehensive, copy-pasteable samples suite (`samples/`) enabling new users to bootstrap kubeOP and deploy workloads end-to-end.
- Cover tenant onboarding, delivery types, operational features, observability, lifecycle, multi-tenancy, and security scenarios.
- Ensure scripts are idempotent, fully logged, and validated via CI on KinD.

### Key Features
- **Directory Layout**: Structured hierarchy from `00-bootstrap/` through `09-security/`, each with `README.md`, `.env.example`, `curl.sh`, `verify.sh`, `cleanup.sh`.
- **Global Environment**: Shared `.env.samples` defining `KUBEOP_BASE_URL`, `AUTH_TOKEN`, `DOMAIN`, `CF_API_TOKEN`, `DEFAULT_CLUSTER_ID`, `STORAGE_FAST`, `STORAGE_BULK`.
- **Bootstrap**: Local KinD demo and real-cluster guidance with logging, `set -euo pipefail`, and explicit step outputs.
- **Tenant & Project Creation** *(✅ Samples under `samples/01-tenant-project` with docs in `docs/samples/01-tenant-project.md`)*: Scripts for tenant provisioning, kubeconfig generation, quotas/limits.
- **Flavors**: Resource presets (small/medium/gpu/storage) with JSON descriptors and application scripts.
- **Delivery Types**: Executable examples for Helm repo/OCI, Git (Helm/Kustomize), raw manifests, OCI bundles including rendered diff and prune verification.
- **Templates**: Schema-backed template registration and app instantiation with advanced input examples.
- **Features & Observability**: HTTPS via Cloudflare, autoscaling, PVC, jobs, network policies, RBAC, logs, events, metrics.
- **Lifecycle & Security**: Rollouts, backups, pruning, multi-tenant isolation, signature verification, policy enforcement.
- **Makefile & CI**: `samples/Makefile` orchestrating flows ✅; GitHub Action running KinD smoke tests.

### Dependencies
- KinD or Talos for local bootstrap.
- jq, curl, kubectl, envsubst, cosign (for signature demos), Helm and kustomize CLIs.
- Access to kubeOP API endpoints and kubeconfig.
- GitHub Actions runner with Docker support for KinD tests.

### Milestones & Deliverables
1. **Scaffolding** *(Done — PR TBD: shared logging helpers and bootstrap sample committed)*
   - Create directory structure with README templates and environment files.
   - Implement logging conventions and helper library (e.g., `samples/lib/common.sh`).
2. **Bootstrap & Tenant Setup**
   - Provide scripts for local KinD deployment and real cluster integration.
   - Document prerequisites and teardown steps.
3. **Delivery Type Examples**
   - Author runnable scripts for each delivery type with verification and cleanup.
   - ✅ Helm repository automation sample in `samples/02-helm-repo/` with documentation at `docs/samples/03-helm-repo.md`.
   - Ensure each example labels resources appropriately.
4. **Feature & Observability Workflows**
   - Implement TLS, autoscaling, PVC, jobs, network policies, RBAC, logs, events, metrics usage.
5. **Lifecycle, Multi-Tenancy, Security**
   - Demonstrate update/rollback, backups, prune, tenant isolation, signature verification, policy gates.
6. **Automation & Testing**
   - Build `samples/Makefile` for orchestration ✅; integrate GitHub Action to run subset on KinD (`helm-repo/nginx`, `git-kustomize/app`).
   - Provide verification scripts using curl + kubectl assertions.

### Implementation Steps
1. Draft documentation guidelines and logging helpers (e.g., `log_step`, `log_info`).
2. Scaffold directories with `README.md` describing objectives and expected outcomes.
3. Author `.env.example` per sample; scripts source `.env.samples` + sample `.env`. *(00-bootstrap ✅)*
4. Implement `curl.sh`, `verify.sh`, `cleanup.sh` with `set -euo pipefail` and explicit log output.
5. Add `verify.sh` checks (HTTP requests, kubectl status, metrics queries).
6. Provide `cleanup.sh` to delete apps and restore quotas.
7. Update `docs/index.md` or `docs/getting-started.md` to reference samples library.
8. Add GitHub Action workflow (`.github/workflows/samples.yml`) using KinD to execute smoke tests.
9. Document sample suite in `README.md` and `docs/guides/` for discoverability.

### Technical Notes & Examples
- Standard script header:
  ```bash
  #!/usr/bin/env bash
  set -euo pipefail
  source "$(dirname "$0")/../../.env.samples"
  source "$(dirname "$0")/.env"
  log_step "Registering tenant"
  ```
- Verification snippet for Helm deployment:
  ```bash
  RELEASE_ID=$(curl -sS -H "Authorization: Bearer $AUTH_TOKEN" \
    -H "Content-Type: application/json" \
    "$KUBEOP_BASE_URL/v1/apps/$APP_ID/releases" | jq -r '.[0].id')
  kubectl --namespace "$NAMESPACE" get deploy "$APP_NAME" -o yaml
  ```
- Ensure CI scripts clean up KinD clusters and Docker resources to keep runs idempotent.

---

## Epoch V — Strategic Streams (A–F)

The fifth epoch evolves into a collection of parallel, strategically aligned streams. Each stream is independently planable and executable while building on the foundations delivered in Epochs I–IV. Teams can start any stream when prerequisites are met, allowing iterative delivery of future-looking capabilities instead of a monolithic "future phase".

### Stream A — Platform Hardening & Governance

#### Goals
- Formalize API contracts, release processes, and guardrails that enable safe, multi-tenant operations.
- Provide administrative tooling and compliance features required for production-grade governance.

#### Key Features
- **Core API Hardening & Versioning**: Introduce semantic versioning, OpenAPI schemas, and deprecation policies.
- **Governance & Billing Hooks**: Per-tenant quotas, pre-billing alerts, policy evaluation, and webhook notifications.
- **Admin Ops Toolbox**: Safe node drains/cordons, audit trails. Maintenance mode toggles ✅ (see [Maintenance mode toggle tutorial](./TUTORIALS/maintenance-mode-toggle.md)).
- **Audit & Compliance**: Tamper-evident logs, retention configuration, export utilities.

#### Dependencies
- Requires Epoch I schema persistence (releases) and Epoch IV operational docs for reference workflows.
- Coordination with security/compliance stakeholders to define acceptable controls and retention periods.

#### Milestones & Deliverables
1. **Contract Definition** *(✅ Completed — see [`docs/reference/api-contract.md`](./reference/api-contract.md) and updated `README.md`/`docs/changelog.md`)*: Document API schemas, changelog policy, and release cadence in `docs/CHANGELOG.md` and `README.md`.
2. **Governance Controls**: Implement quota/policy evaluation services with tests and admin UX or API endpoints.
3. **Audit Enhancements**: Extend logging to include tamper-evident hashing, retention enforcement, and export commands.
4. **Admin Toolkit**: Deliver scripts or endpoints for safe cluster operations with runbooks in `docs/operations/`.

#### Implementation Steps
1. Draft ADRs for API versioning strategy and changelog requirements.
2. ✅ Update `internal/version/version.go` mechanics to support SemVer ranges and deprecation metadata *(Done — compatibility metadata is exposed via `/v1/version` alongside new documentation and logging warnings).* 
3. Build policy evaluation modules referencing metering data for enforcement decisions.
4. Add CLI/REST surfaces for admin controls and provide automated tests covering edge cases.

#### Technical Notes & Examples
- Adopt URL versioning (`/v1`, `/v1beta1`) with JSON Schema validation and contract tests.
- Use append-only audit logs with hash chaining to detect tampering.
- Provide governance alerts via webhooks that tenants can consume for automated billing workflows.

### Stream B — Multi-Cluster Operations & Health

#### Goals
- Manage fleets of clusters with consistent health insights, drift detection, and credential lifecycle management.
- Ensure kubeOP can orchestrate applications across heterogeneous infrastructure.

#### Key Features
- **Multi-Cluster Registry** *(Done — [Cluster inventory walkthrough](./TUTORIALS/cluster-inventory-service.md))*: Catalog clusters, metadata, ownership, and status.
- **Health & Drift Monitoring**: Scheduled probes, configuration drift alerts, remediation hints.
- **Disaster Recovery & Backups**: Backup/restore playbooks for control planes and kubeOP state.

#### Dependencies
- Relies on delivery metadata from Epoch I and observability hooks from Epoch III (metering) for accurate attribution.
- Requires secure credential storage patterns and RBAC conventions established earlier.

#### Milestones & Deliverables
1. **Cluster Inventory Service**: Database schema, APIs, and UI/CLI for registering clusters with status indicators.
3. **Drift Detection**: Implement comparison jobs that highlight configuration drift and propose remediation steps.
4. **Resilience Playbooks**: Publish disaster recovery runbooks and automated backup/restore tooling.

#### Implementation Steps
1. Define cluster metadata schema and migrations; add APIs for registration and credential rotation.
2. Implement gRPC/HTTP streaming from agents with authentication and retry logic.
3. Integrate drift detection with reconciliation history and Git/app templates for expected state.
4. Automate snapshots/backups for kubeOP databases and document recovery validation procedures.

#### Technical Notes & Examples
- Health probes can reuse metrics gathered by kubeop-meter for consistency.
- Drift detection can leverage `kubectl diff` or server-side apply dry-runs against desired manifests.

### Stream C — Developer Experience & Automation

#### Goals
- Streamline workflows for application developers and DevOps teams through automation and integrations.
- Reduce friction when deploying via Git, CI pipelines, or command-line tooling.

#### Key Features
- **Marketplace & App Templates**: Curated catalogs, versioning, and sharing of templates across tenants.
- **Webhooks & CI Triggers**: Git push notifications, manual approvals, canary/staged rollout pipelines.
- **Public SDK/CLI**: Lightweight client for authentication, CRUD operations, logs, and metrics.
- **Kubectl Mirroring & Takeover**: Detect manual Kubernetes changes and absorb them into kubeOP-managed apps.

#### Dependencies
- Requires template infrastructure from Epoch I and samples from Epoch IV to seed catalog content.
- Needs governance controls from Stream A to ensure automated flows respect policy.

#### Milestones & Deliverables
1. **Template Marketplace**: API endpoints and UI/CLI for browsing, subscribing, and publishing templates.
2. **Automation Hooks**: Implement webhook receivers, approval workflows, and deployment pipelines with audit trails.
3. **SDK/CLI Release**: Publish SDK/CLI packages (Go, TypeScript) with docs and automated tests.
4. **Takeover Mechanism**: Develop reconciliation logic to adopt unmanaged resources into kubeOP's delivery model.

#### Implementation Steps
1. Extend template metadata to include sharing scopes and version lineage.
2. Build webhook services that translate Git events into deployment actions with retry and rollback support.
3. Scaffold CLI commands mirroring API operations and integrate with samples for tutorials.
4. Implement detection of unlabeled Kubernetes resources and interactive adoption flows.

#### Technical Notes & Examples
- SDK should support token refresh, pagination helpers, and strongly typed delivery specs.
- Canary rollouts can reuse release tracking to compare metrics before promotion.
- Adoption workflows must honor labels and avoid taking over system namespaces.

### Stream D — Observability, Cost Intelligence & UX

#### Goals
- Enhance visibility into application health, usage, and costs while improving the operator/developer experience.
- Provide cohesive dashboards, logs/events search, and reporting integrations.

#### Key Features
- **Metrics & Metering Enhancements**: Map usage to cost centers, export to Prometheus/ClickHouse/S3, CSV/JSON reports.
- **Logs & Events Experience**: Searchable timelines, retention policies, downloadable archives.
- **Ingress/DNS/TLS Automation**: Automated domain assignment, certificate issuance/renewal, provider integrations.
- **UI / Portal**: Optional web console for tenants and operators featuring dashboards and guided workflows.

#### Dependencies
- Builds on kubeop-meter (Epoch III) and samples/observability flows (Epoch IV) for baseline data sources and scripts.
- Requires governance decisions from Stream A regarding retention, access controls, and cost allocation.

#### Milestones & Deliverables
1. **Cost Attribution Engine**: Enhance meter database with cost mappings and export jobs.
2. **Observability API & UI**: Deliver search APIs and UI views for logs/events/metrics with RBAC.
3. **Ingress Automation**: Implement DNS providers integration (e.g., Cloudflare) and certificate management pipelines.
4. **Experience Polish**: Design and ship web portal or dashboards, update documentation with screenshots and flows.

#### Implementation Steps
1. Extend meter service to compute cost dimensions and publish export APIs.
2. Index logs/events with search backends (e.g., Elasticsearch, Loki) and expose query endpoints.
3. Build automation controllers for DNS/TLS with retries and renewal alerts.
4. Iterate UI wireframes, implement React/Vue front-end (if adopted), and integrate with API authentication.

#### Technical Notes & Examples
- Cost exports can emit to object storage in Parquet/CSV for BI consumption.
- Logs/events search should include pagination, filters, and retention warnings.
- TLS automation must support multiple issuers and handle rate-limit backoff.

### Stream E — Security & Resilience Enhancements

#### Goals
- Strengthen end-to-end security posture and ensure platform resilience against failures and threats.
- Provide tenants with robust secret management, identity controls, and failover strategies.

#### Key Features
- **Identity & Authorization**: Fine-grained RBAC, token lifecycle, kubeconfig rotation/revocation.
- **Secrets & Config Attachments**: Typed attachments, rotation hooks, redaction, sealed secret integrations.
- **Scaling & Resilience Controls**: HPA/KEDA, disruption budgets, automated rollback triggers.
- **Security Hardening**: mTLS between components, JWT rotation, threat modeling, pen-test readiness.

#### Dependencies
- Requires foundational delivery pipelines (Epoch I) and governance frameworks (Stream A).
- May leverage multi-cluster infrastructure (Stream B) for failover strategies.

#### Milestones & Deliverables
1. **Identity Platform**: Implement central identity provider integration, token issuance, and revocation workflows.
2. **Secret Management Enhancements**: Add APIs/CRDs for attachments, rotation automation, and audit logs.
3. **Resilience Tooling**: Ship scaling policies, disruption budgets, and automated rollback commands with documentation.
4. **Security Reviews**: Conduct threat modeling sessions, pen tests, and document remediation plans.

#### Implementation Steps
1. Integrate with OIDC providers for SSO and map roles to kubeOP RBAC.
2. Build secret rotation pipelines with encrypted storage and notification hooks.
3. Implement resilience controllers that observe metrics and trigger rollbacks or scale events.
4. Establish continuous security testing (SAST/DAST) and update incident response playbooks.

#### Technical Notes & Examples
- Use short-lived tokens with refresh flows and audit logging for all identity operations.
- Secret attachments should support file mounts, environment variables, and sealed secret bundles.
- Automated rollbacks can compare release metrics from kubeop-meter before promoting updates.

### Stream F — Job & Schedule Management

#### Goals
- Deliver first-class support for Kubernetes Jobs and CronJobs with lifecycle tracking, logs, and billing parity with Deployments/Apps.
- Provide APIs, UI, and automation that let tenants orchestrate ad-hoc and recurring workloads inside the kubeOP tenancy model.

#### Key Features
- **API & Model Layer**: New `/v1/jobs` endpoints for creation, listing, deletion, and log retrieval plus `job`/`schedule` support in `AppSpec` and a JSON schema for job templates (`image`, `command`, `env`, `ttlSecondsAfterFinished`).
- **Scheduler Support**: Accept cron expressions with timezone and `concurrencyPolicy` controls (`Allow`, `Forbid`, `Replace`) and validate schedules before persistence.
- **User Experience**: Surface job history, real-time logs, and a "Run Now" trigger for CronJobs in the tenant UI and API responses.
- **Billing & Metrics**: Attribute runtime, resource consumption, and exit state per execution and feed data into kubeop-meter for per-run billing.
- **Security & Isolation**: Enforce namespace quotas, security policies, and restrictions on privileged containers or host-level access for batch workloads.
- **Documentation & Samples**: Ship `samples/jobs/simple-job.yaml`, `samples/jobs/cron-job.yaml`, API reference updates, and CLI examples demonstrating creation, monitoring, and cleanup flows. Initial manifest examples now live in [`samples/jobs/`](https://github.com/vaheed/kubeOP/tree/main/samples/jobs) with walkthroughs in [`docs/samples/02-jobs.md`](./samples/02-jobs.md).

#### Dependencies
- Relies on Epoch I delivery metadata for consistent labelling and Epoch III metering for cost attribution.
- Builds on Epoch IV samples infrastructure for runnable examples and Stream A governance controls for quota/policy enforcement.

#### Milestones & Deliverables
1. **API & Schema Enablement**
   - Extend `AppSpec` and models to represent jobs and schedules, expose `/v1/jobs` endpoints, and add JSON schema validation for job templates.
   - Document API contract updates in `docs/reference/` and refresh OpenAPI specs.
   - Implement schedule validation, timezone handling, and concurrency policies in the control plane scheduler.
3. **User Experience & Samples**
   - Add UI panels for job history, run-now actions, and live logs, plus CLI walkthroughs and sample manifests under `samples/jobs/`.
   - ✅ Sample manifests published under [`samples/jobs/`](https://github.com/vaheed/kubeOP/tree/main/samples/jobs) with docs in [`docs/samples/02-jobs.md`](./samples/02-jobs.md).
   - Update billing pipelines to export per-run usage into kubeop-meter and surface metrics in docs and dashboards.

#### Implementation Steps
1. Model and persist job specifications, including TTL and schedule metadata, ensuring migrations and repository updates remain backward compatible.
2. Build handler methods for `/v1/jobs` with validation, log streaming hooks, and structured logging using `internal/logging`.
4. Introduce scheduler utilities for cron parsing (including timezone and concurrency policy) with unit tests covering edge cases and validation errors.
5. Augment billing collectors to ingest per-job runtime/resource usage and publish metrics to kubeop-meter with attribution to tenants, projects, and apps.
6. Produce documentation updates, sample manifests, and UI/CLI screenshots demonstrating creation, monitoring, retries, and cleanup flows.

#### Technical Notes & Examples
- Validate cron expressions during creation and reject malformed schedules before writing to storage.
- Enforce namespace policies so batch workloads inherit tenant quotas and security profiles without requiring privileged containers.
- Example job template payload:
  ```json
  {
    "type": "job",
    "job": {
      "image": "ghcr.io/example/batch:latest",
      "command": ["/bin/process", "--input", "s3://bucket/data"],
      "env": [{"name": "ENV", "value": "prod"}],
      "ttlSecondsAfterFinished": 600
    }
  }
  ```

---

## Epoch VI — Coordination & Reporting

- **Versioning**: Each epoch that introduces behaviour changes must bump `internal/version/version.go` and update `docs/changelog.md`.
- **Testing**: Run `go vet ./...`, `go test ./...`, `go test -count=1 ./testcase`, and `go build ./...` before merging code changes.
- **CI/CD**: Extend `.github/workflows/ci.yml` and related workflows as new components/tests are added.
- **Documentation**: Keep `README.md`, `docs/index.md`, and relevant guides synchronized with implemented features.
- **Stakeholder Updates**: Publish progress summaries per milestone and update this roadmap when epochs ship or plans evolve.

