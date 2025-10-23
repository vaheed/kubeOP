# ROADMAP — kubeOP One-Year Execution Plan

**Scope:** mature kubeOP into a production-grade multi-tenant PaaS with CRD source-of-truth, hardened API, strong DX, and observable operations.  
**Cadence:** continuous delivery with weekly tracking.  
**KPIs:** p95 API latency <150 ms; >95% e2e green; <0.5% failed rollouts; mean time to recovery (MTTR) <15 min.

---

## Phase 1 — Foundation & Security (12 weeks)

### 1. CRD Baseline (3 weeks)
- **Goal:** define `App`, `Project`, `Credential`, `Quota` CRDs as the canonical state.
- **Steps**
  - Draft OpenAPI/CRD schemas (validation, defaults, immutables).
  - Controller: reconcile `App` → rendered manifests (SSA) + conditions.
  - Status: `Ready/Degraded/Progressing`, lastApplied digest, observedGeneration.
  - Labeling: `kubeop.*` canonical labels on all owned objects.
- **Acceptance**
  - `kubectl apply -f samples/app-basic.yaml` → `Ready=True` under 90s.
  - Deleting CRD instance cleans up owned objects with finalizers.

### 2. Delivery Engine V2 (3 weeks)
- Uniform pipeline for Helm repo/OCI, raw manifests, Git, OCI bundle.
- Deterministic render + diff; SBOM digest; pluggable credentials.
- **Acceptance:** identical digest per spec; diff view API ready.

### 3. Security & Hardening (3 weeks)
- Outbound HTTP allow-list + DNS pinning; sanitized paths; JWT scopes.
- Pod Security Admission + NetworkPolicy baselines.
- **Acceptance:** CodeQL/SAST zero "Critical/High"; e2e covers SSRF/LFI.

### 4. Ops Basics (3 weeks)
- Structured logs with request IDs; `/healthz`, `/readyz`, `/metrics`; log rotation.

---

## Phase 2 — Tenancy & Observability (12 weeks)

### 5. Project & Tenancy Automation (3 weeks)
- One-call project bootstrap: namespaces, quotas, limits, policies.
- Per-project service accounts; short-lived kubeconfigs; rotation APIs.
- **Acceptance:** bootstrap <30s; verified via `kubectl auth can-i`.

### 6. Change Detection & Timeline (3 weeks)
- Watchers via operator; normalized events to API.
- Timeline API: append-only events + searchable DB.
- **Acceptance:** manual edit triggers drift event within 10s.

### 7. Metrics & Billing Hooks (3 weeks)
- Prometheus counters; usage rollups; daily cost export.
- **Acceptance:** 30-day retention; per-project CSV export.

### 8. Delivery Validation (3 weeks)
- `/v1/apps/validate` with quota + OPA checks; inline diff preview.

---

## Phase 3 — Jobs, GitOps & Reliability (12 weeks)

### 9. Jobs & Schedules (3 weeks)
- `Job` & `CronJob` CRD specs; history limits; TTL cleanup.
- **Acceptance:** CRON fires ±1m; concurrencyPolicy enforced.

### 10. GitOps Bridge (3 weeks)
- Flux/Argo integration; read-only discovery; namespace conventions.
- **Acceptance:** repo changes deploy via Flux; status mirrored in kubeOP.

### 11. Reliability Engineering (3 weeks)
- Retries/backoff; rollback; GC for orphans.
- **Acceptance:** chaos tests pass; no manual recovery needed.

### 12. Backup & Restore (3 weeks)
- DB dumps; artifact indexes; one-command restore.
- **Acceptance:** RPO/RTO documented; restore verified in staging.

---

## Phase 4 — UI, Policy & GA (12 weeks)

### 13. Admin & Tenant Portals (3 weeks)
- Minimal web UI for admin/tenant workflows; RBAC enforced.
- **Acceptance:** CRUD + rollout actions via UI only.

### 14. Policy & Compliance (3 weeks)
- Org quotas; image allow-lists; namespace rules; audit export.
- **Acceptance:** policy blocks verified via OPA + audit logs.

### 15. Multi-Cluster Scale (3 weeks)
- Worker sharding; circuit-breakers; connection pooling.
- **Acceptance:** p95 API <150 ms at 200 concurrent ops.

### 16. GA Readiness (3 weeks)
- API v1 freeze; migration docs; production guide.
- **Acceptance:** pen-test clean; docs + runbook complete.

---

## Quality Standards
- Unit coverage >80% critical packages.
- e2e suite: CRUD, drift, rollout, rollback, jobs, quotas, policies.
- CI: lint, vet, staticcheck, CodeQL, image scan, SBOM attach.
- Perf: 200 concurrent app applies; DB p95 <50 ms write.
- Docs: versioned, bilingual (EN/FA), with diagrams + runbooks.

---

## Deliverables Summary

| Phase | Duration | Major Deliverables |
|--------|-----------|-------------------|
| Foundation & Security | 12 weeks | CRDs, Delivery Engine V2, Security, Logging |
| Tenancy & Observability | 12 weeks | Bootstrap, Events, Billing, Validation |
| Jobs & Reliability | 12 weeks | Jobs, GitOps, Backup, Resilience |
| UI, Policy & GA | 12 weeks | Portals, Policies, Scale, Docs |

---

**Total Duration:** 48 weeks (1 year)

**Outcome:** kubeOP 1.0 GA — a stable, secure, multi-tenant PaaS with full CRD integration, real-time awareness, observability, and enterprise-ready delivery.

