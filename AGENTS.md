# Agent Operating Guide

These rules apply to the entire repository. Follow them alongside any
explicit task instructions.

## Required review before writing code

1. Read this file, `README.md`, `docs/CHANGELOG.md`, the CI workflow in
   `.github/workflows/ci.yml`, and the repository layout notes in
   `docs/ARCHITECTURE.md`. Make sure you understand how code, tests,
   docs, and migrations fit together before planning a change.
2. Inspect the scopes that your change will touch:
   - `cmd/` and `internal/` contain application code.
   - `internal/store/migrations/` holds golang-migrate SQL files.
   - `testcase/` mirrors packages for tests.
   - Documentation lives in `docs/` (except `README.md` and this file).
3. Confirm whether behaviour, APIs, configuration, or schema will change
   and plan updates for docs, README usage examples, CHANGELOG, tests,
   and version metadata accordingly.

## Coding standards

- Go version must match `go.mod` (1.22). Keep imports organised via
  `gofmt`.
- Prefer Go standard library first; justify new dependencies.
- Logging uses `internal/logging` (zap). Emit contextual logs for new
  steps and propagate errors with context.
- Do not wrap `import` statements in try/catch equivalents. Handle
  errors where they occur.
- Keep configuration, secrets, and credentials outside source code. Use
  environment variables or GitHub Actions secrets.

## Database migrations

- Place migrations in `internal/store/migrations/` with filenames of the
  form `NNNN_description.{up,down}.sql` using zero-padded sequential
  numbers starting at `0001`.
- Every up migration must have a matching down migration.
- Never modify an applied migration; add a new version instead.
- When touching migrations, run the migration tests under
  `testcase/migrations_sql_test.go` and ensure new versions are unique
  and contiguous. Update documentation if the schema or behaviour
  changes.

## Tests and quality gates

- Any code change under `cmd/` or `internal/` requires corresponding test
  updates or additions under `testcase/`.
- Run locally (and document in the PR message):
  - `go vet ./...`
  - `go test ./...`
  - `go test -count=1 ./testcase`
  - `go build ./...`
- Keep tests hermetic; use fakes or mocks instead of external services.
- Ensure `go mod tidy` leaves `go.mod`/`go.sum` clean.

## Documentation and release notes

- Update `README.md` with any behaviour, setup, or usage changes.
- Update relevant docs under `docs/` (API reference, environment, ops,
  etc.).
- Maintain `docs/CHANGELOG.md` following Keep a Changelog format and bump
  `internal/version/version.go` (SemVer) when behaviour or schema
  changes. Note breaking changes explicitly.
- Refresh `docs/ROADMAP.md` when new work is identified or existing items
  complete.

## CI expectations

- `.github/workflows/ci.yml` must always install dependencies, lint with
  `go vet`, run unit tests, build the binaries, and upload build
  artifacts. Keep the workflow updated when tooling or commands change.
- The PR checklist in `.github/pull_request_template.md` must stay in
  sync with repository rules.

## Pull request preparation

- Include meaningful commit messages and keep diffs focused on the task.
- Ensure there are no stray Markdown files outside `docs/`, `README.md`,
  and `AGENTS.md`.
- Before requesting review, re-read this file and confirm all
  requirements are satisfied.

