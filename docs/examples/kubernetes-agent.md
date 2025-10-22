# Kubernetes deployment example

This manifest deploys kubeOP into a Kubernetes cluster. Edit secrets and hostnames before applying.

```bash
kubectl create namespace kubeop
kubectl create secret generic kubeop-config \
  --namespace kubeop \
  --from-literal=ADMIN_JWT_SECRET='change-me' \
  --from-literal=KCFG_ENCRYPTION_KEY='base64-32-byte-key'
kubectl apply -f docs/examples/kubeop-deployment.yaml
```

Verify the rollout:

```bash
kubectl get pods -n kubeop
kubectl get ingress -n kubeop
```

Expose the service through your ingress controller or replace the `Ingress` resource with a `LoadBalancer` service if required.
