# Security Policy

KubeOP is an external, API‑only control plane for managing Kubernetes clusters. This document explains how to report vulnerabilities and how we approach security across the project.

## Report a Vulnerability

- **Preferred**: Open a private report via **GitHub → Security → Report a vulnerability** on this repo.
- **Fallback**: Email `security@vaheed.net` (PGP welcome if you have it).  
  Include: affected version/commit, environment, reproduction steps, impact, and any PoC or logs.

**Acknowledgement**: You’ll receive a response within **72 hours**.  
**Coordination window**: We aim to fix and coordinate disclosure within **90 days** (or sooner for critical issues).

## Supported Versions

- **`main`** branch is supported. Pre‑release tags (0.x) may change rapidly; security fixes land on `main` first.  
- No LTS yet. If you’re packaging KubeOP, track `main` or the latest tag.

## In Scope

- The KubeOP **API** (`cmd/api`, `internal/*`)  
- Helm/manifests and sample deployment assets in this repo  
- Build/CI workflows under `.github/`

**Out of scope** (report to upstreams): Kubernetes itself, PostgreSQL, Helm, Docker/Containerd, OS images, and any third‑party libraries.

## Coordinated Disclosure

1. Private report.  
2. Triage and reproduce (we may request more details).  
3. Fix developed in a private branch when warranted.  
4. CVE requested if impact justifies it.  
5. Security release notes published; reporter credited (opt‑in).

## Hardening Expectations

### Transport & Auth

- **HTTPS by default**. If `ALLOW_INSECURE_HTTP=true` is used, **dev/test only**.  
- Admin endpoints require a valid JWT signed with `ADMIN_JWT_SECRET`. Rotate this secret regularly.  
- Kubeconfigs issued to users should be **scoped** and **revocable**; prefer short‑lived credentials with renewal over long‑lived static tokens.

### Least Privilege

- Apply **namespace‑scoped** RBAC for user workloads.  
- Default **NetworkPolicies**, **ResourceQuotas**, and **Pod Security Admission** profiles must be created on project bootstrap.

### Secrets

- Never commit secrets. Use environment variables, secret stores, or Kubernetes `Secret` objects.  
- Redaction: tokens, passwords, and keys must not be printed in logs. Log review is part of PRs.

### Runtime Security (containers)

- Run as non‑root; read‑only filesystem; drop Linux capabilities; add seccomp `RuntimeDefault`.  
- Expose only the API port; no SSH or shell in images.  
- Build images from minimal/distroless bases and pin **digest** (not just tags).

## Secure Development Lifecycle

### Dependency & Supply Chain

- Enable **GitHub Dependabot** + **Security Alerts**.  
- For Go:  
  - `go mod tidy && go mod verify`  
  - `govulncheck ./...`  
- For docs/UI (if applicable): `npm audit --production` (or `pnpm audit`).  
- Generate an SBOM for releases:  
  - `syft dir:. -o cyclonedx-json > sbom.json`  
- Scan images before publish:  
  - `grype .` and `trivy image <image@digest>`

### Code Quality Gates

- Mandatory checks in CI:  
  - **Build & tests** (`go test ./...`)  
  - **Formatting** (`gofmt -s -w` and fail on diff)  
  - **Static analysis** (`go vet ./...`, `staticcheck ./...`)  
  - **Govulncheck** (fail on high/critical)  
- No debug endpoints or default credentials in production builds.

### Reviews

- Security‑sensitive changes (auth, crypto, token issuance, kubeconfig generation, network boundary) require **two maintainer approvals**.  
- Avoid introducing new external services or telemetry without explicit review.

## Kubernetes‑Specific Guidance

- **Per‑project namespace** (recommended) with labels/annotations required by KubeOP.  
- Enforce **PodDisruptionBudgets**, **resource requests/limits**, and **imagePullPolicy: Always** for internet‑facing apps.  
- Ingress/Service:  
  - Prefer **Ingress + TLS** via cert‑manager.  
  - If `LoadBalancer` is allowed, restrict via policy/quotas and annotate for your LB controller as needed.

## Logging, Metrics, and PII

- Structured logs only. No secrets, tokens, or full kubeconfigs in logs.  
- Audit trail for mutating API calls with actor, project, and request path; redact body fields flagged sensitive.  
- Expose `/metrics` (Prometheus) without sensitive label values.

## Building & Releasing

- Reproducible builds: embed version/commit with `-ldflags`.  
- Tag releases; attach `sbom.json` and image digests.  
- Sign container images (e.g., **cosign**) and release artifacts where possible.

## Reporting Templates

**Email/Advisory template**

```
Subject: [KubeOP Security] <short title>

Impact: (RCE, auth bypass, info leak, DoS, supply-chain, etc.)  
Affected: commit/tag/branch, environment (dev/prod), config flags  
Repro: steps/PoC  
Expected vs Actual:  
Workaround/Mitigations (if any):  
Reporter credit: (name/handle, yes/no)  
```

## Hall of Fame

We credit reporters who help keep KubeOP safe (optional, by request).

## Questions

For general security questions (not vulnerabilities), open a **Discussion** with the `security` category or email `security@vaheed.net`.
