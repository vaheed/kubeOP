## Summary
- [ ] Explain the motivation and context for the change.

## Checklist
- [ ] ✅ Matrix mapping stages 00–04 to code/tests/docs is included (link to doc/section or artifact).
- [ ] ✅ All docs rebuilt (`npm run docs:build`), no legacy files remain, markdown links pass.
- [ ] ✅ `make test-e2e` succeeded locally and artifacts (logs, usage.json, invoice.json, analytics.json) are attached or referenced.
- [ ] ✅ SVG diagrams (`docs/diagrams/*.svg`) are present and referenced from docs when updated.
- [ ] ✅ Helm chart packaged via `make helm-package` (Go-based tooling, artifact archived if applicable).
- [ ] ✅ Security review complete: no request forgery/path injection, secrets handled via env/KMS, admission HA/PDB verified.
- [ ] ✅ GHCR image naming follows `ghcr.io/vaheed/kubeop/<pkg>` (production) and `ghcr.io/vaheed/kubeop/<pkg>-dev` (development), including CI tags, bootstrap overrides, and Compose defaults.
- [ ] ✅ Multi-arch (`linux/amd64`, `linux/arm64`) build settings validated for production images.
- [ ] Tests, lint, and docs updated (`make right`, `go test ./...`, `npm run docs:build`).
- [ ] ✅ GitHub Pages deployment renders with the `/kubeOP/` base path (CSS/assets verified).

## Testing
- [ ] `go test ./...`
- [ ] `make right`
- [ ] `npm run docs:build`
- [ ] `make test-e2e`
