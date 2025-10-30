# Documentation workflow

This site is generated with VitePress using the Markdown files under `docs/`.

## Install dependencies

```bash
cd docs
npm install
```

`npm install` creates `node_modules/` and `package-lock.json` (ignored by Git via the repository `.gitignore`).

## Local preview

```bash
npm run docs:dev
```

This runs `vitepress dev .` and watches the Markdown tree for changes.

## Build for publication

```bash
npm run docs:build
```

The static site is emitted to `docs/.vitepress/dist`. Use `npm run docs:preview` to verify the output locally before pushing.
