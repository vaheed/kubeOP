Development Notes

CI Enforcement

- This repo enforces via CI that any code change under `internal/` or `cmd/` must include:
- An update to at least one Markdown document (under `docs/` or `README.md`/`AGENTS.md`).
  - An update/addition to at least one test file under `testcase/`.
- See `AGENTS.md` for the complete rules that CI validates.

Docs & Site

- Docsify serves from `docs/`.
- Local preview:
  - Docker: `docker run -it --rm -p 3000:3000 -v "$PWD":/site -w /site/docs node:20 npx docsify serve .`
  - Node: `cd docs && npx docsify serve .`

PR Checklist

- [ ] Tests updated in `testcase/` for any code change.
- [ ] Documentation updated (`docs/` or `README.md`) when behavior or usage shifts.
- [ ] `docs/CHANGELOG.md` updated and `internal/version` bumped for releases.
- [ ] README quickstart/usage examples refreshed if commands or flows change.
- [ ] CI workflow expectations met: vet, build, test, and artifact steps succeed locally.
- [ ] New endpoints reflected in `docs/API_REFERENCE.md` and `docs/openapi.yaml` with request/response examples.

OpenAPI

- Spec lives at `docs/openapi.yaml` (OpenAPI 3.0.3). View with ReDoc at `docs/openapi.html`.
- Update the spec when adding/changing endpoints. Checklist:
  - Paths: endpoints, methods, request bodies, responses, status codes
  - Schemas: request/response types, enums, oneOf where applicable
  - Security: bearerAuth on admin endpoints; health/ready/version unsecured
  - Examples: optional but encouraged for complex payloads
