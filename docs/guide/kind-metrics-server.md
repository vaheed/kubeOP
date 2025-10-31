---
outline: deep
---

# Enable HPA on Kind (metrics-server)

The HorizontalPodAutoscaler needs the Kubernetes resource metrics API. On Kind
clusters, the kubelet uses a self-signed certificate, so you must pass relaxed
flags to metrics-server.

## Install metrics-server (automatic via bootstrap)

Set an environment variable before running the Kind bootstrap script and it
will install and patch metrics-server automatically:

```bash
export KUBEOP_INSTALL_METRICS_SERVER=true
make kind-up
bash e2e/bootstrap.sh
```

## Install metrics-server (manual)

```bash
# Install the latest published components
kubectl apply -f \
  https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# Patch args for Kind (insecure TLS + preferred address types)
kubectl -n kube-system patch deploy metrics-server --type='json' -p='[
  {"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-insecure-tls"},
  {"op":"add","path":"/spec/template/spec/containers/0/args/-","value":"--kubelet-preferred-address-types=InternalIP,Hostname,InternalDNS,ExternalDNS,ExternalIP"}
]'

# Wait for metrics-server to become Ready
kubectl -n kube-system rollout status deploy/metrics-server --timeout=120s

# Verify the API is responding
kubectl top nodes || true
kubectl top pods -A || true
```

## Enable HPA for the operator (optional)

Once metrics-server is healthy, you can enable the HPA in the kubeOP chart and
use relaxed thresholds for development:

```bash
helm upgrade --install kubeop-operator charts/kubeop-operator -n kubeop-system \
  --create-namespace \
  --set hpa.enabled=true \
  --set hpa.minReplicas=1 \
  --set hpa.maxReplicas=3 \
  --set hpa.targetCPUUtilizationPercentage=50
```

> Note: Keep HPA disabled in CI unless you also install metrics-server there.
> Do not use `--kubelet-insecure-tls` in production clusters.
