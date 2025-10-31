# PROMPTS — 00 to 30

> Each prompt includes **Prerequisites**, **Goal**, **Deliverables**, **CI/CD**, and **Test Plan**. Implement in order unless your team parallelizes per ROADMAP/ZOOM-MAP.

## 00 — Kind E2E Harness (First!)
Prerequisites: none
Goal: One‑command CI env (Kind + Compose) to boot platform and run E2E.
Deliverables: Makefile (`kind-up`, `platform-up`, `manager-up`, `test-e2e`, `down`), `kind-config.yaml`, `hack/e2e` tests, DNS/ACME mocks, GH Actions job.
CI/CD: run `make test-e2e` on PR; upload logs/artifacts.
Test Plan: tenant→project→app→DNS/TLS→usage→invoice→analytics checks pass.

## 01 — Manager API & DB (Go + Postgres)
Prerequisites: 00
Goal: REST API for clusters/tenants/projects/apps/policies/kubeconfigs/usage/invoices/dns/certs/hooks.
Deliverables: OpenAPI, migrations, JWT+RBAC, KMS/envelope for secrets.
CI/CD: vet/staticcheck/tests; build & push; release on tag.
Test Plan: CRUD + webhooks + invoice export.

## 02 — In‑Cluster Operator (Controllers)
Prerequisites: 00,01
Goal: CRDs: Tenant/Project/App/Policy/Registry/DNSRecord/Certificate; reconcile namespaces, quotas, NP, registry, egress allow.
Deliverables: status conditions, events, revision persistence.
CI/CD: chart packaging; e2e on Kind.
Test Plan: create T/P/App with host; DNS/TLS OK; Git update rolls out; rollback works.

## 03 — Admission Webhooks
Prerequisites: 00,01,02
Goal: Mutate labels/namespace; validate cross‑tenant refs, quotas, image allowlists, egress baseline.
Deliverables: HA webhooks (PDB), CA bundle patch.
CI/CD: e2e negative tests.
Test Plan: forbidden scenarios denied; required labels injected.

## 04 — Delivery Controller
Prerequisites: 00,01,02,03
Goal: Render/apply Git/Helm/Image/Raw; revision history; rollout strategies; probes/presets defaults; hooks pre/post.
Deliverables: SSA apply; status.revision; rollout events.
CI/CD: integration with demo repo.
Test Plan: push → redeploy; rollback snapshot; hooks run once.

## 05 — Git Webhook & Poller
Prerequisites: 00,01,04
Goal: HMAC‑verified webhook; dedup queue; optional poller; retries.
Deliverables: `/hooks/github`, DLQ.
CI/CD: webhook simulation in CI.
Test Plan: push triggers reconcile; missed webhook caught by poller.

## 06 — Per‑Project Registry
Prerequisites: 00,01,02,03
Goal: attach private registry; image allowlists; imagePullSecrets management.
Deliverables: Project.spec.registry; Admission validation.
CI/CD: sample CRs; e2e allowed/denied images.
Test Plan: success with allowed, deny otherwise.

## 07 — Egress Firewall & Accounting
Prerequisites: 00,01,02,03
Goal: default‑deny egress; allowlists; per‑container bytes via eBPF.
Deliverables: egress agent; Reporter integration.
CI/CD: policy tests; synthetic traffic.
Test Plan: curl blocked→allowlisted works; bytes reported hourly.

## 08 — Reporter & Billing
Prerequisites: 00,01,02
Goal: hourly usage → invoices (RateCard YAML).
Deliverables: usage ingestion; biller aggregation; CSV/JSON export.
CI/CD: math tests; idempotency.
Test Plan: synthetic usage produces expected totals.

## 09 — Kubeconfig Issuer & RBAC
Prerequisites: 00,01,02,03
Goal: role‑scoped kubeconfigs; audit issuance; short‑lived tokens optional.
Deliverables: Role/Binding templates; API to issue/renew.
CI/CD: e2e for read‑only vs write.
Test Plan: viewer blocked on writes; admins OK.

## 10 — DNS & Certificates
Prerequisites: 00,01,02,04
Goal: automate A/CNAME and ACME (HTTP‑01/DNS‑01) with custom domains via CNAME/TXT validation.
Deliverables: dns‑manager, certs‑manager; Operator `DNSRecord`/`Certificate`.
CI/CD: mocks in Kind; chart packaging.
Test Plan: host → A → cert → HTTPS OK.

## 11 — Autoscaling Foundation
Prerequisites: 00,01,02,03,04,08
Goal: HPAs for platform + apps; guardrails.
Deliverables: HPAs, PDB, backoff; App.spec.autoscale.
CI/CD: load tests; stability checks.
Test Plan: scale up/down under load.

## 12 — Analytics Data Plane (No UI)
Prerequisites: 00,01,08
Goal: TSDB + API (`/analytics/query` & `/analytics/rollup`), retention 1d/1w/1m.
Deliverables: prom proxy; curated rollups; labels {tenant,project,app,cluster}.
CI/CD: smoke queries.
Test Plan: rollup parity with PromQL.

## 13 — App Templates Marketplace
Prerequisites: 00,02,04,06
Goal: Template/TemplateCatalog; install to App with safe defaults.
Deliverables: sample nginx/wordpress templates; versioning.
CI/CD: lint charts; e2e install/upgrade/rollback.
Test Plan: template lifecycle succeeds.

## 14 — Event Notifications
Prerequisites: 00,01,02,04,08
Goal: webhooks/email on deploy/fail/quota/invoice/cert‑expiring/dns‑error.
Deliverables: NotificationChannel/Policy; retries + DLQ.
CI/CD: routing tests.
Test Plan: events fire & delivered.

## 15 — Tenant Peering (Optional)
Prerequisites: 00,02,03,07,09
Goal: opt‑in private connectivity with TTL and port scoping.
Deliverables: Peering CRD → symmetric NPs/gateway.
CI/CD: leak tests.
Test Plan: only peered traffic allowed; TTL expiry revokes.

## 16 — Backup & Restore
Prerequisites: 00,01,02
Goal: namespace backups to S3; restore.
Deliverables: Velero integration or native; schedules; encryption.
CI/CD: nightly backup; restore test.
Test Plan: parity after restore.

## 17 — Audit Logs
Prerequisites: 00,01,03
Goal: collect Kubernetes Audit/Admission; correlate; search API.
Deliverables: parsers; storage; metrics counters.
CI/CD: PII redaction tests.
Test Plan: actions searchable.

## 18 — App Health Score
Prerequisites: 00,02,08,12
Goal: 0–100 health with availability/restarts/error/latency.
Deliverables: series + alerts; `/apps/:id/health`.
CI/CD: scoring tests.
Test Plan: crashes lower score; recovery raises.

## 19 — Optional Service Mesh
Prerequisites: 00,02,07,12
Goal: Linkerd‑style opt‑in mTLS/policy.
Deliverables: MeshPolicy; injection; metrics.
CI/CD: soak tests.
Test Plan: mTLS verified; policies enforced.

## 20 — Secrets Integration (Vault/SOPS)
Prerequisites: 00,01,02,04
Goal: external secrets without plaintext.
Deliverables: ESO or native; Admission tenancy checks.
CI/CD: e2e with Vault/SOPS.
Test Plan: secret delivered; cross‑tenant denied.

## 21 — Build from Source (Buildpacks/Kaniko)
Prerequisites: 00,01,04,06
Goal: build image from Git repo; push to project registry; feed Delivery.
Deliverables: build job (Tekton/Job) + cache; SBOM artifact.
CI/CD: PR builds; cache reuse.
Test Plan: source‑only app becomes running Deployment.

## 22 — ServiceBinding & EmailBinding
Prerequisites: 00,02,03,04
Goal: standard bindings for DB/Cache/Object Storage/SMTP.
Deliverables: CRDs + operator wiring to inject Secrets/Env.
CI/CD: samples + e2e.
Test Plan: app consumes binding; rotation updates pods.

## 23 — DB Templates (PG/MySQL/Redis)
Prerequisites: 00,02,04,16
Goal: safe templates with backups by default.
Deliverables: charts/templates; PVC; readiness probes; backup hooks.
CI/CD: deploy + backup smoke.
Test Plan: template deploys; snapshot exists.

## 24 — Storage Policies (SC/Resize/Snapshots)
Prerequisites: 00,02,03
Goal: project storage class policy; Admission checks; PVC resize/snapshots.
Deliverables: controller & webhooks; docs.
CI/CD: e2e resize; snapshot tests.
Test Plan: disallowed sizes rejected; resize works.

## 25 — Probes & Resource Presets
Prerequisites: 00,04
Goal: defaults for readiness/liveness/startup + S/M/L resource presets.
Deliverables: delivery injects sane defaults (override allowed).
CI/CD: probe smoke tests.
Test Plan: apps recover; presets respected.

## 26 — Deploy Hooks & Migrations
Prerequisites: 00,04
Goal: preDeploy/postDeploy hooks; db migrations.
Deliverables: Job templates; idempotency; timeouts.
CI/CD: migration tests.
Test Plan: hook runs once; failures rollback.

## 27 — Cost Preview API
Prerequisites: 00,01,08,12
Goal: “what‑if” estimate for an App before deploy.
Deliverables: `/apps/plan` using rate card + resources; returns monthly/hourly.
CI/CD: snapshot tests.
Test Plan: estimates consistent with invoices (± small delta).

## 28 — Per‑Tenant NAT & Rate Limits
Prerequisites: 00,02,07,12
Goal: map egress per tenant to dedicated IP/rate; better tracking/limits.
Deliverables: egress gateway config; metrics; quotas.
CI/CD: traffic shaping tests.
Test Plan: tenant A limited; tenant B unaffected.

## 29 — Ephemeral Debug Access
Prerequisites: 00,01,03,17
Goal: time‑boxed `kubectl debug` for project‑admin with audit.
Deliverables: Admission annotations; RBAC; TTL controller; audit events.
CI/CD: expiry tests.
Test Plan: debug pod auto‑removed; audit logged.

## 30 — Supply Chain Security Policy
Prerequisites: 00,01,06
Goal: SBOM, sign, scan → Admission verify; policy gates on severity.
Deliverables: CI steps (Trivy/Grype + Cosign); Admission policy; exceptions process.
CI/CD: failing scan blocks PR; provenance attached.
Test Plan: unsigned image denied; signed allowed; critical vulns block deploy.
