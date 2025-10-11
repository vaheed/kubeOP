API Quickstart (Step by Step)

Goal

- Go from an empty system to a running app, then clean up.
- Commands are copy/paste ready. Replace placeholders in <>.

Auth setup

- Admin token: export `AUTH_H="-H 'Authorization: Bearer $TOKEN'"` with a JWT signed by `ADMIN_JWT_SECRET` and claim `{"role":"admin"}`.
- For local testing, you can set `DISABLE_AUTH=true` and omit `AUTH_H`.

1) Register a cluster (kubeconfig_b64 required)

- Linux/macOS: `B64=$(base64 -w0 < kubeconfig)`
- Windows (PowerShell): `$B64 = [Convert]::ToBase64String([IO.File]::ReadAllBytes('kubeconfig'))`
- Create cluster:
  - `curl -s $AUTH_H -H 'Content-Type: application/json' -d "$(jq -n --arg n 'my-cluster' --arg b64 "$B64" '{name:$n,kubeconfig_b64:$b64}')" http://localhost:8080/v1/clusters`
- Save the `id` field as `<cluster-id>`.

2) Create user (bootstrap user namespace and get kubeconfig)

- By email (create or reuse):
  - `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"name":"Alice","email":"alice@example.com","clusterId":"<cluster-id>"}' http://localhost:8080/v1/users/bootstrap`
- Or by existing userId:
  - `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"userId":"<user-id>","clusterId":"<cluster-id>"}' http://localhost:8080/v1/users/bootstrap`
- Save:
  - `USER_ID=<value from .user.id>`
  - `NS=<value from .namespace>`
  - `echo "$(jq -r '.kubeconfig_b64')" | base64 -d > user.kubeconfig`

3) Create project

- `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"userId":"'$USER_ID'","clusterId":"<cluster-id>","name":"demo"}' http://localhost:8080/v1/projects`
- Save `PROJECT_ID=$(jq -r '.project.id')`

4) Create app (image)

- Use an unprivileged image and high container port (PSA restricted):
  - `curl -s $AUTH_H -H 'Content-Type: application/json' -d '{"name":"web","image":"nginxinc/nginx-unprivileged:1.27-alpine","ports":[{"containerPort":8080,"servicePort":80,"serviceType":"LoadBalancer"}]}' http://localhost:8080/v1/projects/$PROJECT_ID/apps`
- Save `APP_ID=$(jq -r '.appId')`
- Check objects:
  - `KUBECONFIG=./user.kubeconfig kubectl -n $NS get deploy,svc -o wide`

5) Delete app

- API: `curl -s $AUTH_H -X DELETE http://localhost:8080/v1/projects/$PROJECT_ID/apps/$APP_ID`

6) Delete project

- `curl -s $AUTH_H -X DELETE http://localhost:8080/v1/projects/$PROJECT_ID`

7) Delete user

- API: `curl -s $AUTH_H -X DELETE http://localhost:8080/v1/users/$USER_ID`
- Behavior: soft-delete the user in DB; delete user namespaces across clusters.

Listings (IDs lookup)

- Users: `curl -s $AUTH_H http://localhost:8080/v1/users | jq`
- Clusters: `curl -s $AUTH_H http://localhost:8080/v1/clusters | jq`
- Projects (all): `curl -s $AUTH_H http://localhost:8080/v1/projects | jq`
- Projects by user: `curl -s $AUTH_H http://localhost:8080/v1/users/$USER_ID/projects | jq`

Notes

- LoadBalancer external IP requires a provider (e.g., MetalLB). Otherwise, use ClusterIP and port-forward.
- User kubeconfigs are namespace-scoped. Use `-n $NS` for kubectl.
- For per-project namespaces, set `PROJECTS_IN_USER_NAMESPACE=false` and `POST /v1/projects` will return `kubeconfig_b64` for the project.
