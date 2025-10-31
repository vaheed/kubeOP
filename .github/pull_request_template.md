## Summary
- [ ] Explain the motivation and context for the change.

## Checklist
- [ ] ✅ Stage-to-code/test/doc mapping documented (link to doc section or artifact).
- [ ] ✅ Docs rebuilt when touched (`npm run docs:build`), links validated.
- [ ] ✅ CI-only e2e harness (`hack/e2e`) verified via workflow artefacts (logs, dumps, coverage) – attach links in the PR description.
- [ ] ✅ SVG diagrams (`docs/diagrams/*.svg`) updated when diagrams change.
- [ ] ✅ Helm chart packaging (`make helm-package`) performed when chart manifests change.
- [ ] ✅ Security review complete: no request forgery/path injection, secrets handled via env/KMS, admission HA/PDB verified.
- [ ] ✅ GHCR image naming follows `ghcr.io/vaheed/kubeop/<pkg>` (production) and `ghcr.io/vaheed/kubeop/<pkg>-dev` (development), including CI tags, bootstrap overrides, and Compose defaults.
- [ ] ✅ Multi-arch (`linux/amd64`, `linux/arm64`) build settings validated for production images.
- [ ] Tests, lint, and docs updated (`go test -tags=short ./...`, `go test -tags=integration ./...`, `npm run docs:build`).
- [ ] ✅ GitHub Pages deployment renders with the `/kubeOP/` base path (CSS/assets verified).
- [ ] ✅ CI `summary` job reviewed (coverage ≥ 80%, no unretried failures, flakes noted).

## Testing
- [ ] `go test -tags=short ./...`
- [ ] `go test -tags=integration ./...`
- [ ] `go test -tags=e2e ./hack/e2e -count=1 -timeout 90m` (runs CI harness; ensure Kind prerequisites present if running locally)
- [ ] `npm run docs:build`
