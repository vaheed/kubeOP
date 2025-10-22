# Documentation style guide

The kubeOP documentation aims to be direct, task-focused, and technically precise. Follow this guide when writing or updating
content.

## Voice and tone

- Use plain English, present tense, and active voice.
- Address the reader as "you" when giving instructions.
- Keep sentences short (≤ 25 words) and avoid idioms.
- Prefer consistent terminology: *kubeOP*, *control plane*, *tenant*, *project*, *App CRD*.

## Structure

- Start each page with an H1 heading (`# Title`). Subsequent sections use `##` and `###` in order.
- Provide a short introduction (1–2 sentences) explaining the page purpose.
- Use ordered lists for sequential steps, unordered lists for options.
- Include a summary table when listing configuration keys, API fields, or troubleshooting symptoms.

## Code and configuration examples

- Use fenced code blocks with a language identifier:
  ```markdown
  ```bash
  docker compose up -d
  ```
  ```
- For inline commands, wrap them in backticks (for example, `kubectl get pods`).
- Show complete, copy-pasteable snippets. Include environment preparation commands when required.

## Callouts and tables

- Use the VitePress syntax for callouts:
  ```markdown
  ::: tip
  Helpful hint.
  :::
  ```
  Available types: `note`, `tip`, `caution`, `warning`.
- Tables must include a header row and align with Markdownlint rule `MD033` by avoiding raw HTML.

## Linking

- Prefer relative links within the repository (for example, `[Quickstart](QUICKSTART.md)`).
- External links must use HTTPS and include descriptive link text.
- Avoid bare URLs; always wrap them in `<...>` or Markdown link syntax.

## File placement

- Place user-facing documentation under `docs/`.
- Store reusable fragments in `docs/_snippets/` and media in `docs/media/`.
- Keep governance documents at the repository root.

## Linting

Two tools enforce the style guide:

- **markdownlint** – configured via `.markdownlint.json`. Run with `npm run docs:lint`.
- **Vale** – configured via `.vale.ini` and custom styles under `.github/vale/styles/`. Run with `npm run docs:lint`.

Address all lint errors before opening a pull request.
