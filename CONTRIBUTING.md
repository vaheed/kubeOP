# Contributing to kubeOP

Thank you for investing your time in improving kubeOP. This guide explains how to set up a development environment, follow the
project conventions, and submit high-quality pull requests.

## Getting started

1. **Fork and clone**

   ```bash
   git clone https://github.com/<your-username>/kubeOP.git
   cd kubeOP
   git remote add upstream https://github.com/vaheed/kubeOP.git
   ```

2. **Install prerequisites**
   - Go 1.24.3+
   - Node.js 20+
   - Docker & Docker Compose (for integration smoke tests)
   - PostgreSQL 14+ (local instance or container)
3. **Bootstrap tooling**

   ```bash
   go mod download
   npm ci
   ```

4. **Run the test suite** to confirm the baseline state

   ```bash
   go vet ./...
   go test -count=1 ./...
   go test -count=1 ./testcase
   go build ./...
   npm run docs:build
   ```

## Development workflow

- **Code organisation**
  - `cmd/api/` – HTTP server entrypoint.
  - `internal/` – application services, stores, and integrations.
  - `kubeop-operator/` – controller-runtime manager.
  - `testcase/` – black-box tests aligned with package boundaries.
  - `docs/` – VitePress content, style guide, and reusable snippets.
- **Branching** – base your work on `main`. Use topic branches named `<type>/<short-description>` (for example,
  `docs/new-quickstart`).
- **Style** – run `gofmt` on Go code, follow [`docs/STYLEGUIDE.md`](docs/STYLEGUIDE.md) for documentation, and keep logging
  contextual using `internal/logging`.
- **Dependencies** – prefer the standard library. If you must add a dependency, document the justification in the pull request
  and ensure `go mod tidy` remains clean.

## Commit messages

- Write clear, imperative subject lines (≤ 72 characters).
- Use body paragraphs to explain the motivation and noteworthy changes.
- Reference issues using `Fixes #123` when closing tickets.

## Pull request checklist

Before requesting review:

- [ ] Rebase on the latest `main` and resolve conflicts.
- [ ] Run `go vet ./...`, `go test ./...`, `go test -count=1 ./testcase`, and `go build ./...`.
- [ ] Run `npm run docs:lint` and `npm run docs:build`.
- [ ] Update `README.md`, relevant docs, and `CHANGELOG.md` when behaviour, configuration, or user workflows change.
- [ ] Add or update tests under `testcase/` for code changes.
- [ ] Ensure `.github/workflows/ci.yml` continues to install dependencies, lint, test, build, and upload artifacts.
- [ ] Ensure logging includes meaningful context for new operations.

## Code review expectations

- Respond to feedback promptly and courteously.
- Keep follow-up commits focused; squash or tidy before merging.
- Document any trade-offs or known gaps in the pull request description.

## Reporting issues

Use [GitHub Issues](https://github.com/vaheed/kubeOP/issues) for bug reports and feature requests. Include:

- kubeOP version (`/v1/version` output)
- Environment (Kubernetes version, cloud provider, OS)
- Steps to reproduce or reproduction repository
- Expected vs actual behaviour

Security reports should follow the process described in [`SUPPORT.md`](SUPPORT.md).
