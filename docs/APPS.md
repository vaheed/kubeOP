Apps

- Deploy via Docker image, Helm chart (tarball URL), or plain manifests using the API.
- Creates a Deployment, a Service (ClusterIP or LoadBalancer), and optionally an Ingress if a domain is provided or wildcard is enabled.

Endpoints

- POST `/v1/templates` — Register templates (helm/manifests/blueprints)
  - Request: `{ "name":"nginx", "kind":"manifests", "spec": {"docs": ["...yaml..."] } }`

- POST `/v1/projects/{id}/apps` — Deploy app into a project namespace
  - Common request fields:
    - `name` (string)
    - `flavor` (optional: `f1-small|f2-medium|f3-large`)
    - `resources` (optional) — override requests/limits
    - `replicas` (optional)
    - `env` (optional map), `secrets` (optional array of Secret names)
    - `ports` (optional) — maps container→Service; set `serviceType` to `ClusterIP` or `LoadBalancer`
    - `domain` (optional explicit host)
    - `repo` (optional repository identifier for CI webhooks)
    - `webhookSecret` (optional per-app secret for webhook signature verification)
  - Source options (exactly one):
    - Docker image: `{ "image":"nginx:1.27" }`
    - Manifests: `{ "manifests":["apiVersion: v1\nkind: ConfigMap\n..."] }`
    - Helm: `{ "helm": {"chart":"https://.../grafana-<ver>.tgz", "values":{}} }`

Behavior

- If `domain` is omitted and `PAAS_WILDCARD_ENABLED=true`, host is `{app}.{namespace}.{PAAS_DOMAIN}` and an Ingress is created targeting HTTP (80/8080 if present).
- `flavor` sets sensible defaults; `resources` override them.
- `services.loadbalancers` quota is enforced against `MAX_LOADBALANCERS_PER_PROJECT` or project overrides.

How-to: Deploy from Docker Hub

- One-liner (LB + wildcard ingress):
  - `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"name":"web","image":"nginx:1.27","ports":[{"containerPort":80,"servicePort":80,"serviceType":"LoadBalancer"}]}' http://localhost:8080/v1/projects/<project-id>/apps`

End-to-end: Docker image

- 1) Register cluster → `clusterId` (docs/API_REFERENCE.md:55)
- 2) Bootstrap user → save kubeconfig (docs/API_REFERENCE.md:23)
- 3) Create project → `project.id`
- 4) Deploy image:
  - `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"name":"api","image":"nginx:1.27","env":{"FOO":"bar"},"ports":[{"containerPort":80,"servicePort":80}]}' http://localhost:8080/v1/projects/<project-id>/apps`
- 5) Access:
  - Wildcard ingress: `http://api.<namespace>.<PAAS_DOMAIN>` if enabled
  - Or `kubectl -n <namespace> get svc api -o wide` for external IP

How-to: Deploy via Helm (Grafana)

- Provide the chart tarball URL and values (approved minimal support):
  - `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"name":"grafana","helm":{"chart":"https://grafana.github.io/helm-charts/grafana-8.5.13.tgz","values":{"adminUser":"admin","adminPassword":"StrongPassw0rd!"}}}' http://localhost:8080/v1/projects/<project-id>/apps`
- Notes:
  - Supported input: direct `https://.../*.tgz` URLs. OCI charts and repo+chart resolving may be added later.

How-to: Deploy from a Git repo

 - Option A (manifests): render manifests in your CI and POST them as `manifests`.
 - Option B (image + webhooks):
   - Deploy an image and associate repo + secret:
     - `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"name":"api","image":"org/api:latest","repo":"org/repo","webhookSecret":"<hmac-secret>","ports":[{"containerPort":8080,"servicePort":80}]}' http://localhost:8080/v1/projects/<project-id>/apps`
   - In your Git provider, add a push webhook to `http://<kubeop>/v1/webhooks/git` with header `X-Hub-Signature-256: sha256=<hex(hmac(body, secret))>`.
   - On push, KubeOP patches a Deployment annotation to trigger a rollout for matching repo.

Security defaults (PSA "restricted")

- Image-based deploys set secure container defaults:
  - runAsNonRoot=true, allowPrivilegeEscalation=false
  - seccompProfile=runtime/default, capabilities.drop=[ALL]
  - readOnlyRootFilesystem=true
- Use images that run as a non-root user, and prefer containerPort > 1024.
- If your image requires root or a writable root FS, adjust the image or supply manifests/helm with your own securityContext.

Delete apps

- API: `curl -s $AUTH_H -X DELETE http://localhost:8080/v1/projects/<project-id>/apps/<appId>` → `{"status":"deleted"}`
- Behavior: soft-delete the app in DB; delete labeled Kubernetes resources (Deployments/Services/Ingresses/Jobs/CronJobs/ConfigMaps/Secrets/PVCs) in the project namespace.

Grafana end-to-end (ready to run)

- 1) Register cluster: see docs/API_REFERENCE.md:55 (kubeconfig_b64 required)
- 2) Bootstrap user: see docs/API_REFERENCE.md:23 → save kubeconfig
- 3) Create project: `POST /v1/projects` with `{userId, clusterId, name:"grafana"}`
- 4) Deploy via Helm (tarball URL):
  - `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"name":"grafana","helm":{"chart":"https://grafana.github.io/helm-charts/grafana-8.5.13.tgz","values":{"adminUser":"admin","adminPassword":"StrongPassw0rd!"}}}' http://localhost:8080/v1/projects/<project-id>/apps`
- 5) Logs: `curl -s $AUTH_H http://localhost:8080/v1/projects/<project-id>/apps/<appId>/logs?tailLines=200`
- 6) Access:
  - If wildcard ingress configured, open `http://grafana.<namespace>.<PAAS_DOMAIN>`
  - Or get Service external IP with: `KUBECONFIG=./user.kubeconfig kubectl -n <namespace> get svc grafana -o wide`
