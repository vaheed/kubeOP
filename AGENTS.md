# kubeOP — Agents Guide
Simple, professional rules for working in this repository.

## Tools & Environment
- **Core stack:** Go `1.24+`, Docker, Docker Compose, Kind, `kubectl`, Helm, `jq`, `yq`
- **Docs:** Node.js `18+` (VitePress)
- **Database:** PostgreSQL (local or CI service)
- Keep your local setup consistent with the CI pipeline.
- **License:** MIT — ensure documentation and headers reflect this when touched.

## Branching Model
- **`main`** → Production branch  
  Builds multi-arch images: `ghcr.io/vaheed/kubeop/<pkg>:latest` (linux/amd64, linux/arm64)
- **`develop`** → Active development  
  Builds dev images: `ghcr.io/vaheed/kubeop/<pkg>-dev:dev`
- Never push directly to `main`. Always create a PR from a feature branch and always push to `develop`.

## E2E Testing Layout
All end-to-end assets are located in `e2e/`:
- Config: `kind-config.yaml`
- Manifests: `e2e/k8s/`
- Scripts: `run.sh`, `bootstrap.sh`

**Common targets:**
```bash
make kind-up         # Start local Kind cluster
make platform-up     # Bootstrap kubeOP platform components
make manager-up      # Deploy operator
make test-e2e        # Run E2E suite
make down            # Tear down environment
```

## Extended E2E Testing  
Every change must be validated in both **Kind** (local) and **real clusters** (staging or production).  
- The Kind-based E2E ensures quick local reproducibility.  
- Real cluster E2E validates integration with actual infrastructure, CRDs, RBAC, and networking.  
- CI automatically runs Kind tests; real cluster E2E can be triggered manually via a pipeline flag (`E2E_REAL_CLUSTER=true`).  
- All tests must pass in both environments before tagging a release.  
- Test coverage includes tenant/project/app lifecycle, delivery, admission webhooks, and billing metrics.

## Testing Rules
- Place unit tests as `*_test.go` next to their packages.  
  Use `tests/` only for black-box tests.
- Postgres-backed tests auto-skip if the database is unavailable.
- CI provides a Postgres service for integration tests.
- Every new module, endpoint, or logic addition must include coverage.
- Before committing:
  ```bash
  go test ./... && make right
  ```

## Code & Style
- Keep commits focused, small, and composable.
- Follow Go idioms and run:
  ```bash
  make right
  ```
  This runs `fmt`, `vet`, `tidy`, and build checks.
- Avoid hard-coded secrets; use environment variables.
- Every service must expose `/healthz` and `/metrics`.
- Follow 12-factor app configuration principles.

## Documentation
- Docs live under `docs/` (VitePress):
  ```bash
  npm install
  npm run docs:dev
  ```
- Update the matching guide in `docs/` when behavior changes.
- Internal work notes → `docs/internal/WORKLOG.md`
- Major feature or API changes → update `docs/ROADMAP.md`, `CHANGELOG.md`, and bump `VERSION`
- Only one `README.md` in root; all other markdown files go under `docs/`

## CI/CD
- Single workflow: `.github/workflows/ci.yaml`
- Pipeline stages: **Test → Build → E2E → Push**
- CI builds and pushes Docker images to GHCR.
- Tag strategy:
  - `main` → push `latest`, `sha-<short>`, and semantic tags for `ghcr.io/vaheed/kubeop/<pkg>`
  - `develop` → push `dev` and `sha-<short>` tags for `ghcr.io/vaheed/kubeop/<pkg>-dev`

## Secrets & Environment
- Copy `.env` from [`env.example`](./env.example).
- Each variable must have a clear comment and sensible default.
- Example:
  ```bash
  KUBEOP_DB_URL=postgres://user:pass@localhost:5432/kubeop
  KUBEOP_JWT_SIGNING_KEY=<base64-key>
  KUBEOP_REQUIRE_AUTH=true
  ```
- Never commit credentials or real secrets.

## Best Practices
- Always test locally before opening a PR.
- Keep PRs small and single-purpose.
- Use conventional commit types: `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`
- Write code that’s simple, maintainable, and self-explanatory.
- Prefer clarity over cleverness — simplicity scales.