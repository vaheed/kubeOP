# Contributing

- Read `AGENTS.md` for repository rules (tests, docs, changelog, CI updates).
- Follow Go formatting (`gofmt`) and keep imports tidy.
- Update documentation alongside behaviour changes (README, guides, API pages).
- Run `go vet ./...`, `go test ./...`, `go test -count=1 ./testcase`, and `npm run docs:build` before submitting PRs.
- Keep changelog entries in [`docs/changelog.md`](changelog.md) and bump `internal/version/version.go` for releases.
- The PR template and CI workflow (`.github/workflows/ci.yml`) outline mandatory checks.
