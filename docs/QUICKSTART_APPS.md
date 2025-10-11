Quickstart: Apps

Goal

- Get from an empty system to a running app (Docker image, Helm, or Git) via the API with copy/paste commands.

Prereqs

- Admin token with claim {"role":"admin"} → export `AUTH_H="-H 'Authorization: Bearer $TOKEN'"`
- Database running and API up (see README.md:1 and docs/OPERATIONS.md:1)

1) Register a cluster (base64 kubeconfig)

- Linux/macOS: `B64=$(base64 -w0 < kubeconfig)`
- Windows: `$B64 = [Convert]::ToBase64String([IO.File]::ReadAllBytes('kubeconfig'))`
- `curl -s $AUTH_H -H 'Content-Type: application/json' -d "$(jq -n --arg n 'my-cluster' --arg b64 "$B64" '{name:$n,kubeconfig_b64:$b64}')" http://localhost:8080/v1/clusters`
- Save `id` as `<cluster-id>`

2) Bootstrap user namespace (shared mode)

- `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"name":"Alice","email":"alice@example.com","clusterId":"<cluster-id>"}' http://localhost:8080/v1/users/bootstrap`
- Save `user.id`, `namespace`, and write kubeconfig: `echo "<kubeconfig_b64>" | base64 -d > user.kubeconfig`

3) Create a project

- `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"userId":"<user-id>","clusterId":"<cluster-id>","name":"demo"}' http://localhost:8080/v1/projects`
- Save `project.project.id` as `<project-id>` and `project.project.namespace` as `<ns>`

4A) Deploy from Docker image (Docker Hub)

- `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"name":"web","image":"nginx:1.27","ports":[{"containerPort":80,"servicePort":80,"serviceType":"LoadBalancer"}]}' http://localhost:8080/v1/projects/<project-id>/apps`
- Logs: `curl -s $AUTH_H http://localhost:8080/v1/projects/<project-id>/apps/<appId>/logs?tailLines=200`
- Access:
  - Wildcard ingress (enable with env): `http://web.<ns>.<PAAS_DOMAIN>`
  - Or external IP: `KUBECONFIG=./user.kubeconfig kubectl -n <ns> get svc web -o wide`

4B) Deploy via Helm (Grafana)

- `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"name":"grafana","helm":{"chart":"https://grafana.github.io/helm-charts/grafana-8.5.13.tgz","values":{"adminUser":"admin","adminPassword":"StrongPassw0rd!"}}}' http://localhost:8080/v1/projects/<project-id>/apps`
- Access similarly to 4A.

4C) Deploy from Git (image + webhook)

- Deploy app linked to repo:
  - `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"name":"api","image":"org/api:latest","repo":"org/repo","webhookSecret":"<hmac-secret>","ports":[{"containerPort":8080,"servicePort":80}]}' http://localhost:8080/v1/projects/<project-id>/apps`
- In Git provider, add push webhook → `http://<kubeop>/v1/webhooks/git` with header `X-Hub-Signature-256: sha256=<hex(hmac(body, secret))>`.
- On push, rollout is triggered.

Environment knobs (ingress/LB/DNS)

- `PAAS_DOMAIN`, `PAAS_WILDCARD_ENABLED=true` → generate `{app}.{namespace}.{PAAS_DOMAIN}` hosts
- `LB_DRIVER=metallb`, optional `LB_METALLB_POOL`
- `MAX_LOADBALANCERS_PER_PROJECT` (default 1) or project override `services.loadbalancers`
- Optional DNS upsert to map host → LB IP:
  - `EXTERNAL_DNS_PROVIDER=cloudflare` + `CF_API_TOKEN`, `CF_ZONE_ID` (or `powerdns` + `PDNS_*`)

What next?

- Deeper docs:
  - Apps: docs/APPS.md:1
  - CI Webhooks: docs/CI_WEBHOOKS.md:1
  - Ingress/LB: docs/INGRESS_LB.md:1
  - Environment: docs/ENVIRONMENT.md:1
  - API: docs/API_REFERENCE.md:1 and docs/openapi.yaml:1

