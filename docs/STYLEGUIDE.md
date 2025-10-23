# Documentation style guide

The kubeOP documentation should be concise, technically accurate, and approachable for operators. Follow these rules whenever
you write or update content.

## Voice and tone

- Use present tense and active voice. Prefer short sentences (≤ 25 words).
- Address the reader as “you” in procedural content.
- Spell the product name as **kubeOP** (capital OP) and keep Kubernetes resource names in TitleCase (for example, Deployment).
- Avoid idioms, future tense speculation, and rhetorical questions.

## Structure and headings

- Every page starts with an H1 (`# Title`). Increase heading levels sequentially (`##`, then `###`).
- Open with a short introduction explaining why the reader should care.
- Use ordered lists for procedures, unordered lists for options or attributes.
- Add tables when referencing configuration keys, API fields, or environment matrices. Tables must have a header row.

## Code, commands, and snippets

- Use fenced code blocks with explicit language identifiers (`bash`, `yaml`, `json`, `mermaid`).
- Provide full, copy-pasteable commands that include prerequisite exports or environment preparation.
- Wrap inline commands and filenames in backticks.
- Reuse shared content from `docs/_snippets/` by embedding with VitePress includes:
  ```markdown
  ::: include docs/_snippets/curl-auth.md
  :::
  ```
- When referencing sample manifests or scripts, link to the files under `docs/examples/`.

## Callouts and notes

Use VitePress callouts to emphasise key information:

```markdown
::: note
Background context or prerequisites.
:::

::: tip
Helpful hints or best practices.
:::

::: caution
Warnings about irreversible or high-risk actions.
:::
```

## Terminology

Maintain consistent vocabulary:

| Term | Use | Avoid |
| --- | --- | --- |
| kubeOP | The control plane project. | kubeop, KubeOp |
| tenant | A logical owner of projects and namespaces. | customer, client |
| project | The unit of application deployment. | workspace, app namespace |
| App CRD | The in-cluster custom resource managed by the operator. | kubeOPApp |

## Linking and references

- Prefer relative links for in-repo assets (for example, `[Quickstart](QUICKSTART.md)`).
- External links must use HTTPS and descriptive link text.
- Do not leave bare URLs; wrap them in Markdown links or angle brackets.
- Cite diagrams with descriptive alt text explaining what the reader should notice.

## Diagrams and media

- Store Mermaid snippets under `docs/_snippets/diagram-*.md` as fenced `mermaid` code blocks.
- Embed diagrams with `::: include docs/_snippets/<file>.md` so the same source renders in multiple pages.
- Do not commit rendered PNG/SVG assets. GitHub and VitePress render Mermaid at build time.
- Mention the key takeaway in the paragraph preceding each diagram.

## Linting and review

- Run `npm run docs:lint` before committing. The command ensures Vale is installed (via `go install`) and runs it with `.vale.ini`
  for terminology and style checks.
- Address Vale warnings; do not add ignore directives without reviewer approval.
- Run `npm run docs:build` to validate includes and Mermaid rendering after updating diagrams.

## Pull request expectations

- Update `README.md`, `docs/`, and `CHANGELOG.md` whenever behaviour or usage changes.
- Mention doc linting and Mermaid validation in the PR checklist when applicable.
- Provide copy-paste verification commands for reviewers (Quickstart, API calls, etc.).
