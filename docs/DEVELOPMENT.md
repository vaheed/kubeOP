Development Notes

CI Enforcement

- This repo enforces via CI that any code change under `internal/` or `cmd/` must include:
  - An update to at least one Markdown document (under `documents/` or `README.md`/`AGENTS.md`).
  - An update/addition to at least one test file under `testcase/`.
- See `AGENTS.md` for the complete rules that CI validates.

