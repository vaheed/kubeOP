---
outline: deep
---

# Production Install Guide

This guide covers a production‑grade install on an existing Kubernetes cluster using:

- kubeOP operator + admission chart (OCI)
- cert-manager (Let’s Encrypt Issuer)
- ExternalDNS with PowerDNS
- Image digests, non‑root, PDBs, and metrics scraping

## Prerequisites

- Kubernetes v1.26+
- kubectl + Helm 3.13+
- Access to GHCR (images/charts)
- DNS zone managed by PowerDNS (or substitute your DNS provider)

## 1) Install cert-manager

```bash
helm repo add jetstack https://charts.jetstack.io
helm repo update
kubectl create namespace cert-manager
helm install cert-manager jetstack/cert-manager \
  -n cert-manager \
  --set crds.enabled=true
```

Create a production ClusterIssuer (Let’s Encrypt):

```yaml
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    email: you@example.com
    server: https://acme-v02.api.letsencrypt.org/directory
    privateKeySecretRef:
      name: le-account-key
    solvers:
      - http01:
          ingress:
            class: nginx
```

> Tip: For DNS‑01 (wildcards), use a Cloudflare DNS solver in the Issuer instead.

## 2) Install ExternalDNS (PowerDNS)

Create a secret with your PowerDNS API key (PDNS_API_KEY) in kube-system:

```bash
kubectl -n kube-system create secret generic pdns-credentials \
  --from-literal=api-key='<PDNS_API_KEY>'
```

Add the official ExternalDNS chart and install with provider=powerdns. Set the PowerDNS API endpoint (HTTP) and domain filter for your zone.

```bash
helm repo add external-dns https://kubernetes-sigs.github.io/external-dns/
helm repo update

helm upgrade --install external-dns external-dns/external-dns -n kube-system \
  --set provider=powerdns \
  --set env[0].name=PDNS_API_KEY \
  --set env[0].valueFrom.secretKeyRef.name=pdns-credentials \
  --set env[0].valueFrom.secretKeyRef.key=api-key \
  --set extraArgs[0]=--pdns-server=http://powerdns.example.com:8081 \
  --set domainFilters[0]=example.com \
  --set policy=sync \
  --set txtOwnerId=kubeop \
  --set interval=1m
```

## 3) Set security envs (Manager + Admission)

Set strict defaults in the manager environment (Compose, Helm values, or Kubernetes Secret):

```env
KUBEOP_IMAGE_ALLOWLIST=docker.io,ghcr.io
KUBEOP_EGRESS_BASELINE=10.0.0.0/8,172.16.0.0/12,192.168.0.0/16
KUBEOP_QUOTA_MAX_REQUESTS_CPU=4
KUBEOP_QUOTA_MAX_REQUESTS_MEMORY=8Gi
```

## 4) Install kubeOP operator + admission (OCI chart)

Pin images by digest (recommended). Retrieve digests from CI output or `docker buildx imagetools inspect`.

```bash
NAMESPACE=kubeop-system
helm upgrade --install kubeop-operator oci://ghcr.io/vaheed/charts/kubeop-operator \
  -n $NAMESPACE --create-namespace \
  -f charts/kubeop-operator/values-prod.yaml \
  --set image.digest=sha256:<OPERATOR_DIGEST> \
  --set admission.image.digest=sha256:<ADMISSION_DIGEST> \
  --set mocks.enabled=false
```

Verify readiness:

```bash
kubectl -n $NAMESPACE rollout status deploy/kubeop-operator --timeout=180s
kubectl -n $NAMESPACE rollout status deploy/kubeop-admission --timeout=180s
```

## 5) Expose metrics (Prometheus)

If you run Prometheus Operator, enable ServiceMonitor in `values-prod.yaml`. Otherwise, scrape the Service directly and restrict access via NetworkPolicy.

## 6) DNS + TLS flow

In production, prefer cert-manager (Certificate) and ExternalDNS (Ingress/Service) to manage real DNS/TLS.

Example Ingress (TLS via cert-manager, DNS via ExternalDNS):

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: web
  namespace: kubeop-tenant1-web
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-prod
    external-dns.alpha.kubernetes.io/hostname: web.prod.example.com
spec:
  tls:
    - hosts: [web.prod.example.com]
      secretName: web-tls
  rules:
    - host: web.prod.example.com
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: web
                port:
                  number: 80
```

> The minimal kubeOP DNSRecord/Certificate reconcilers are designed for demos. For production, rely on cert-manager and ExternalDNS to provision real materials; use kubeOP for orchestration/guardrails.

## 7) Hardening

- Enable leader election and PDBs (values-prod.yaml)
- Pin images by digest (operator and admission)
- Restrict egress to baseline CIDRs; allowlisted registries only
- RBAC: issue project‑scoped JWTs and namespace‑scoped kubeconfigs from the Manager

## 8) Upgrade

Upgrade the chart with new digests/tags:

```bash
helm upgrade kubeop-operator oci://ghcr.io/vaheed/charts/kubeop-operator \
  -n kubeop-system -f charts/kubeop-operator/values-prod.yaml \
  --set image.digest=sha256:<NEW_OPERATOR_DIGEST> \
  --set admission.image.digest=sha256:<NEW_ADMISSION_DIGEST>
```
