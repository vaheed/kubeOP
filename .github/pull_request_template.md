## Summary
- [ ] Describe the change and reference the packages or manifests it touches.

## Checklist
- [ ] README and `/docs` content regenerated from the current Go sources (no stale sections or legacy files).
- [ ] `go test ./...` executed and passing.
- [ ] `make right` (fmt, vet, tidy, build) executed and passing.
- [ ] `npm run docs:build` executed to validate the VitePress site.
- [ ] Examples under `examples/` updated or confirmed current when behavior changes.
- [ ] `CHANGELOG.md` and `VERSION` updated if release notes are required.
- [ ] CI configuration covers dependency install, lint, test, build, and artifact upload.

## Testing
- [ ] `go test ./...`
- [ ] `make right`
- [ ] `npm run docs:build`
