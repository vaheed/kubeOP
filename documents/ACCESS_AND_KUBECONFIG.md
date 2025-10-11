Access And Per-User Kubeconfig

Overview

- Current version stores clusters and users in Postgres but does not automatically provision Kubernetes namespaces/RBAC or generate per-user kubeconfigs.
- This guide explains manual steps. Future versions can expose API endpoints to automate them.

Manual Steps (kubectl)

1) Create a namespace for the user (example: user alice on cluster <cluster>):
   - `kubectl create namespace user-alice`

2) Create a service account in that namespace:
   - `kubectl -n user-alice create serviceaccount alice`

3) Grant minimal permissions (example: list/get pods only in user-alice):
   - Role (rbac-user.yaml):
     ```yaml
     apiVersion: rbac.authorization.k8s.io/v1
     kind: Role
     metadata:
       name: alice-role
       namespace: user-alice
     rules:
       - apiGroups: [""]
         resources: ["pods"]
         verbs: ["get", "list", "watch"]
     ```
   - RoleBinding:
     ```yaml
     apiVersion: rbac.authorization.k8s.io/v1
     kind: RoleBinding
     metadata:
       name: alice-rb
       namespace: user-alice
     subjects:
       - kind: ServiceAccount
         name: alice
         namespace: user-alice
     roleRef:
       kind: Role
       name: alice-role
       apiGroup: rbac.authorization.k8s.io
     ```
   - Apply: `kubectl apply -f rbac-user.yaml && kubectl apply -f rolebinding.yaml`

4) Get a token for the service account (Kubernetes 1.24+):
   - `TOKEN=$(kubectl -n user-alice create token alice)`

5) Build a kubeconfig for the user (example using cluster info from current context):
   - `CLUSTER=$(kubectl config view -o jsonpath='{.clusters[0].name}')`
   - `SERVER=$(kubectl config view -o jsonpath='{.clusters[0].cluster.server}')`
   - `CA=$(kubectl config view --raw -o jsonpath='{.clusters[0].cluster.certificate-authority-data}')`
   - Create kubeconfig file:
     ```bash
     cat > alice-kubeconfig <<EOF
     apiVersion: v1
     kind: Config
     clusters:
     - cluster:
         certificate-authority-data: ${CA}
         server: ${SERVER}
       name: ${CLUSTER}
     contexts:
     - context:
         cluster: ${CLUSTER}
         namespace: user-alice
         user: alice
       name: ${CLUSTER}
     current-context: ${CLUSTER}
     users:
     - name: alice
       user:
         token: ${TOKEN}
     EOF
     ```

Validation

- `KUBECONFIG=./alice-kubeconfig kubectl -n user-alice get pods`

Planned API (Future)

- POST `/v1/clusters/{id}/users/{user_id}/provision` — create namespace, service account, role, and binding.
- GET `/v1/clusters/{id}/users/{user_id}/kubeconfig` — return a generated base64 kubeconfig for the service account.
- GET `/v1/clusters/{id}/users/{user_id}/validate` — verify that namespace and RBAC exist and are bound.

