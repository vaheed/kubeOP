# kubectl Command Reference

The zero-to-production guide pairs every API mutation with Kubernetes validation. The commands below are grouped by perspective so you can reuse them during automation and troubleshooting.

## Operator (admin) lens – `--kubeconfig "$TARGET_KUBECONFIG"`

| Purpose | Command |
| --- | --- |
| Confirm cluster access before registration | `kubectl --kubeconfig "$TARGET_KUBECONFIG" get ns` |
| Verify user namespace creation | `kubectl --kubeconfig "$TARGET_KUBECONFIG" get ns "$USER_NAMESPACE"` |
| Inspect tenant ResourceQuota defaults | `kubectl --kubeconfig "$TARGET_KUBECONFIG" -n "$USER_NAMESPACE" get resourcequota tenant-quota -o json | jq '.spec.hard'` |
| Inspect tenant LimitRange defaults | `kubectl --kubeconfig "$TARGET_KUBECONFIG" -n "$USER_NAMESPACE" get limitrange tenant-limits -o json | jq '.spec.limits'` |
| Confirm project namespace provisioning | `kubectl --kubeconfig "$TARGET_KUBECONFIG" get ns "$PROJECT_NAMESPACE"` |
| Review project ResourceQuota | `kubectl --kubeconfig "$TARGET_KUBECONFIG" -n "$PROJECT_NAMESPACE" get resourcequota tenant-quota -o json | jq '.spec.hard'` |
| List project workloads and ingress | `kubectl --kubeconfig "$TARGET_KUBECONFIG" -n "$PROJECT_NAMESPACE" get deploy,svc,ingress` |
| Monitor deployment rollout | `kubectl --kubeconfig "$TARGET_KUBECONFIG" -n "$PROJECT_NAMESPACE" rollout status deploy/hello-nginx --timeout=2m` |
| Inspect cert-manager Certificate status | `kubectl --kubeconfig "$TARGET_KUBECONFIG" -n "$PROJECT_NAMESPACE" get certificate` |
| Capture Service load balancer IP | `kubectl --kubeconfig "$TARGET_KUBECONFIG" -n "$PROJECT_NAMESPACE" get svc hello-nginx -o jsonpath='{.status.loadBalancer.ingress[0].ip}'` |
| List bound ConfigMaps and Secrets | `kubectl --kubeconfig "$TARGET_KUBECONFIG" -n "$PROJECT_NAMESPACE" get configmap,secret` |
| Locate the newest app pod | `kubectl --kubeconfig "$TARGET_KUBECONFIG" -n "$PROJECT_NAMESPACE" get pods -l kubeop.app-id="$APP_ID" -o jsonpath='{.items[0].metadata.name}'` |
| Inspect container environment after attachments | `kubectl --kubeconfig "$TARGET_KUBECONFIG" -n "$PROJECT_NAMESPACE" exec "$POD_NAME" -- env | grep -E 'WELCOME_MESSAGE|SECRET_API_KEY'` |
| Review recent namespace events | `kubectl --kubeconfig "$TARGET_KUBECONFIG" -n "$PROJECT_NAMESPACE" get events --sort-by=.lastTimestamp | tail -n10` |
| Check available replica count after scaling | `kubectl --kubeconfig "$TARGET_KUBECONFIG" -n "$PROJECT_NAMESPACE" get deploy hello-nginx -o jsonpath='{.status.availableReplicas}'` |
| Observe additional Helm/manifest workloads | `kubectl --kubeconfig "$TARGET_KUBECONFIG" -n "$PROJECT_NAMESPACE" get deploy,sts,po,job` |
| Inspect rollout annotation from webhook | `kubectl --kubeconfig "$TARGET_KUBECONFIG" -n "$PROJECT_NAMESPACE" get deploy hello-nginx -o jsonpath='{.metadata.annotations.kubeop\.io/redeploy}'` |
| List workloads prior to project deletion | `kubectl --kubeconfig "$TARGET_KUBECONFIG" -n "$PROJECT_NAMESPACE" get all` |
| Verify namespace removal | `kubectl --kubeconfig "$TARGET_KUBECONFIG" get ns "$PROJECT_NAMESPACE" || echo "namespace deleted"` |
| Verify user namespace removal | `kubectl --kubeconfig "$TARGET_KUBECONFIG" get ns "$USER_NAMESPACE" || echo "user namespace deleted"` |

## Tenant lens – `--kubeconfig alice.kubeconfig`

| Purpose | Command |
| --- | --- |
| Validate RBAC permissions | `kubectl --kubeconfig alice.kubeconfig auth can-i create deployments -n "$USER_NAMESPACE"` |
| Confirm Secret access scope | `kubectl --kubeconfig alice.kubeconfig auth can-i get secrets -n "$USER_NAMESPACE"` |

## Project-scoped lens – `--kubeconfig project-admin.kubeconfig`

| Purpose | Command |
| --- | --- |
| Operate inside the dedicated project namespace | `kubectl --kubeconfig project-admin.kubeconfig -n "$PROJECT_NAMESPACE" get all` |

Reuse these commands whenever you need to cross-check kubeOP API operations against actual cluster state.
