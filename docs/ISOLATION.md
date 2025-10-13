Isolation (NetworkPolicy, Pod Security)

NetworkPolicy

- Default deny for ingress and egress in each project namespace.
- Allow egress to cluster DNS via label selectors:
  - Namespace: `DNS_NS_LABEL_KEY`=`DNS_NS_LABEL_VALUE` (default: `kubernetes.io/metadata.name=kube-system`)
  - Pod: `DNS_POD_LABEL_KEY`=`DNS_POD_LABEL_VALUE` (default: `k8s-app=kube-dns`)
- Allow ingress from namespaces labeled with `INGRESS_NS_LABEL_KEY=INGRESS_NS_LABEL_VALUE` (default: `kubeop.io/ingress=true`). In shared-namespace mode, apply these policies when provisioning the user namespace (external process). In per-project mode (default), they are applied to each project namespace.

Pod Security Admission

- Namespace is labeled with `pod-security.kubernetes.io/enforce=<level>`.
- Set via `POD_SECURITY_LEVEL` env (default `baseline`; switch to `restricted` to enforce non-root containers).
