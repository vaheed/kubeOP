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

OpenAPI

- Spec lives at `docs/openapi.yaml` (OpenAPI 3.0.3). View with ReDoc at `docs/openapi.html`.
- Update the spec when adding/changing endpoints. Checklist:
  - Paths: endpoints, methods, request bodies, responses, status codes
  - Schemas: request/response types, enums, oneOf where applicable
  - Security: bearerAuth on admin endpoints; health/ready/version unsecured
  - Examples: optional but encouraged for complex payloads
