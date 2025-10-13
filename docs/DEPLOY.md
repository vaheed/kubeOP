> **What this page explains**: How to stand up kubeOP locally and in CI.
> **Who it's for**: Operators shipping kubeOP to clusters or GitHub Pages.
> **Why it matters**: Gives repeatable commands to launch the platform everywhere.

# Deployment guide

kubeOP ships as a Docker Compose bundle and a single Go binary. Choose the path that matches your environment and automate it from day one.

## Docker Compose

```yaml
version: "3.9"
services:
  api:
    build: .
    environment:
      - DATABASE_URL=postgres://kubeop:kubeop@postgres:5432/kubeop?sslmode=disable
      - ADMIN_JWT_SECRET=${ADMIN_JWT_SECRET}
      - KCFG_ENCRYPTION_KEY=${KCFG_ENCRYPTION_KEY}
      - LOGS_ROOT=/var/log/kubeop
    ports:
      - "8080:8080"
    depends_on:
      - postgres
  postgres:
    image: postgres:14
    environment:
      - POSTGRES_DB=kubeop
      - POSTGRES_USER=kubeop
      - POSTGRES_PASSWORD=kubeop
    volumes:
      - pgdata:/var/lib/postgresql/data
volumes:
  pgdata: {}
```

### Steps
1. Export strong secrets for `ADMIN_JWT_SECRET` and `KCFG_ENCRYPTION_KEY`.
2. Run `LOGS_ROOT=$PWD/logs docker compose up -d --build`.
3. Tail logs with `docker compose logs -f api` and wait for `readyz`.

## Binary deployment

### Build once
Use `make build` or `go build -o kubeop-api ./cmd/api`. Package the binary with a `config.env` that exports the same environment variables used in Compose.

### Systemd snippet

```ini
[Unit]
Description=kubeOP API
After=network.target postgresql.service

[Service]
EnvironmentFile=/etc/kubeop/config.env
ExecStart=/usr/local/bin/kubeop-api
Restart=on-failure
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

## GitHub Actions

### Workflow outline
Use a two-job workflow: Go build/test and Docker publish, already captured in `.github/workflows/ci.yml`. Add a third job to deploy docs using VitePress.

```yaml
name: docs
on:
  push:
    branches: [main]
  workflow_dispatch: {}
permissions:
  contents: read
  pages: write
  id-token: write
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 20
      - run: npm ci
      - run: npm run docs:build
      - uses: actions/upload-pages-artifact@v3
        with:
          path: docs/.vitepress/dist
  deploy:
    environment:
      name: github-pages
      url: ${{ steps.deploy.outputs.page_url }}
    runs-on: ubuntu-latest
    needs: build
    steps:
      - uses: actions/deploy-pages@v4
        id: deploy
```

### Secrets
Store JWT and encryption secrets in repository secrets. The workflow references them through environment variables and never writes them to logs.

