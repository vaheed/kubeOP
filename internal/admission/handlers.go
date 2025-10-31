package admission

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "net"
    "os"
    "strings"

    admissionv1 "k8s.io/api/admission/v1"
    corev1 "k8s.io/api/core/v1"
    netv1 "k8s.io/api/networking/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    schema "k8s.io/apimachinery/pkg/runtime/schema"
    "k8s.io/apimachinery/pkg/api/resource"
    "k8s.io/client-go/dynamic"
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/rest"
)

var (
    cfgOnce *rest.Config
    k8sOnce *kubernetes.Clientset
    dynOnce dynamic.Interface
)

func restCfg() *rest.Config {
    if cfgOnce == nil {
        c, _ := rest.InClusterConfig()
        cfgOnce = c
    }
    return cfgOnce
}
func kube() *kubernetes.Clientset {
    if k8sOnce == nil { k8sOnce, _ = kubernetes.NewForConfig(restCfg()) }
    return k8sOnce
}
func dyn() dynamic.Interface {
    if dynOnce == nil { dynOnce, _ = dynamic.NewForConfig(restCfg()) }
    return dynOnce
}

func ServeMutate(w http.ResponseWriter, r *http.Request) {
    admit := func(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
        // Default: allow
        resp := &admissionv1.AdmissionResponse{UID: ar.Request.UID, Allowed: true}
        // Only mutate Apps in our API group
        if ar.Request.Kind.Group == "paas.kubeop.io" && strings.EqualFold(ar.Request.Kind.Kind, "App") {
            var obj map[string]any
            if err := json.Unmarshal(ar.Request.Object.Raw, &obj); err == nil {
                patch := make([]map[string]any, 0, 2)
                // ensure metadata exists
                meta, _ := obj["metadata"].(map[string]any)
                labels, labelsOK := meta["labels"].(map[string]any)
                if !labelsOK || labels == nil {
                    // add labels map with our key
                    patch = append(patch, map[string]any{"op": "add", "path": "/metadata/labels", "value": map[string]string{"app.kubeop.io/managed-by": "kubeop-admission"}})
                } else {
                    if _, ok := labels["app.kubeop.io/managed-by"]; !ok {
                        patch = append(patch, map[string]any{"op": "add", "path": "/metadata/labels/app.kubeop.io~1managed-by", "value": "kubeop-admission"})
                    }
                }
                if len(patch) > 0 {
                    b, _ := json.Marshal(patch)
                    pt := admissionv1.PatchTypeJSONPatch
                    resp.PatchType = &pt
                    resp.Patch = b
                }
            }
        }
        return resp
    }
    serve(w, r, admit)
}

func ServeValidate(w http.ResponseWriter, r *http.Request) {
    admit := func(ar admissionv1.AdmissionReview) *admissionv1.AdmissionResponse {
        resp := &admissionv1.AdmissionResponse{UID: ar.Request.UID, Allowed: true}
        // Validate image allowlist and cross-tenant via namespace labels for Apps
        if ar.Request.Kind.Group == "paas.kubeop.io" && strings.EqualFold(ar.Request.Kind.Kind, "App") {
            var obj struct{ Metadata struct{ Namespace string `json:"namespace"` }; Spec struct{ Image string `json:"image"` } }
            if err := json.Unmarshal(ar.Request.Object.Raw, &obj); err == nil {
                // image allowlist
                if host := imageHost(obj.Spec.Image); host != "" {
                    if !allowedRegistry(host) {
                        resp.Allowed = false
                        resp.Result = &metav1.Status{Message: fmt.Sprintf("registry %s is not allowed", host)}
                        return resp
                    }
                }
                // cross-tenant: ensure ns has labels and matches prefix
                if obj.Metadata.Namespace != "" {
                    ns, err := kube().CoreV1().Namespaces().Get(context.Background(), obj.Metadata.Namespace, metav1.GetOptions{})
                    if err == nil {
                        t := ns.Labels["app.kubeop.io/tenant"]
                        p := ns.Labels["app.kubeop.io/project"]
                        if t == "" || p == "" || ns.Name != fmt.Sprintf("kubeop-%s-%s", t, p) {
                            resp.Allowed = false
                            resp.Result = &metav1.Status{Message: "namespace not owned by a tenant/project"}
                            return resp
                        }
                    }
                }
            }
        }
        if ar.Request.Kind.Group == "paas.kubeop.io" && strings.EqualFold(ar.Request.Kind.Kind, "Project") {
            var obj struct{ Spec struct{ TenantRef string `json:"tenantRef"` } }
            if err := json.Unmarshal(ar.Request.Object.Raw, &obj); err == nil {
                if obj.Spec.TenantRef == "" {
                    resp.Allowed = false
                    resp.Result = &metav1.Status{Message: "spec.tenantRef is required"}
                    return resp
                }
                // tenant existence check is best-effort; ignore error
                gvr := schema.GroupVersionResource{Group: "paas.kubeop.io", Version: "v1alpha1", Resource: "tenants"}
                _, err := dyn().Resource(gvr).Get(context.Background(), obj.Spec.TenantRef, metav1.GetOptions{})
                if err != nil {
                    resp.Allowed = false
                    resp.Result = &metav1.Status{Message: "referenced tenant does not exist"}
                    return resp
                }
            }
        }
        // Validate NetworkPolicy egress CIDRs against baseline allowlist (deny internet)
        if ar.Request.Kind.Group == "networking.k8s.io" && strings.EqualFold(ar.Request.Kind.Kind, "NetworkPolicy") {
            var np netv1.NetworkPolicy
            if err := json.Unmarshal(ar.Request.Object.Raw, &np); err == nil {
                // only enforce in kubeOP managed namespaces
                ns, _ := kube().CoreV1().Namespaces().Get(context.Background(), np.Namespace, metav1.GetOptions{})
                if ns != nil && ns.Labels["app.kubeop.io/tenant"] != "" {
                    allows := parseCIDRs(os.Getenv("KUBEOP_EGRESS_BASELINE"))
                    for _, eg := range np.Spec.Egress {
                        for _, to := range eg.To {
                            if to.IPBlock != nil {
                                if !cidrAllowed(to.IPBlock.CIDR, allows) {
                                    resp.Allowed = false
                                    resp.Result = &metav1.Status{Message: fmt.Sprintf("egress CIDR %s not in baseline allowlist", to.IPBlock.CIDR)}
                                    return resp
                                }
                                for _, ex := range to.IPBlock.Except {
                                    if !cidrAllowed(ex, allows) {
                                        resp.Allowed = false
                                        resp.Result = &metav1.Status{Message: fmt.Sprintf("egress except CIDR %s not in baseline allowlist", ex)}
                                        return resp
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
        // Strict ResourceQuota validation (must define requests.cpu and requests.memory, and cap within max env if provided)
        if ar.Request.Kind.Group == "" && strings.EqualFold(ar.Request.Kind.Kind, "ResourceQuota") {
            var rq corev1.ResourceQuota
            if err := json.Unmarshal(ar.Request.Object.Raw, &rq); err == nil {
                rl := rq.Spec.Hard
                if rl == nil || rl.Cpu() == nil || rl.Memory() == nil {
                    resp.Allowed = false
                    resp.Result = &metav1.Status{Message: "resourcequota must set requests.cpu and requests.memory"}
                    return resp
                }
                if maxCPU := os.Getenv("KUBEOP_QUOTA_MAX_REQUESTS_CPU"); maxCPU != "" {
                    if !quantityLEQ(rl[corev1.ResourceRequestsCPU], maxCPU) {
                        resp.Allowed = false
                        resp.Result = &metav1.Status{Message: fmt.Sprintf("requests.cpu exceeds maximum %s", maxCPU)}
                        return resp
                    }
                }
                if maxMem := os.Getenv("KUBEOP_QUOTA_MAX_REQUESTS_MEMORY"); maxMem != "" {
                    if !quantityLEQ(rl[corev1.ResourceRequestsMemory], maxMem) {
                        resp.Allowed = false
                        resp.Result = &metav1.Status{Message: fmt.Sprintf("requests.memory exceeds maximum %s", maxMem)}
                        return resp
                    }
                }
            }
        }
        return resp
    }
    serve(w, r, admit)
}

func serve(w http.ResponseWriter, r *http.Request, f func(admissionv1.AdmissionReview) *admissionv1.AdmissionResponse) {
    var review admissionv1.AdmissionReview
    if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    resp := admissionv1.AdmissionReview{TypeMeta: review.TypeMeta}
    resp.Response = f(review)
    w.Header().Set("Content-Type", "application/json")
    _ = json.NewEncoder(w).Encode(resp)
}

func imageHost(img string) string {
    if img == "" { return "" }
    parts := strings.SplitN(img, "/", 2)
    if len(parts) == 1 { return "docker.io" }
    return parts[0]
}

func allowedRegistry(host string) bool {
    allow := os.Getenv("KUBEOP_IMAGE_ALLOWLIST")
    if allow == "" { return true }
    for _, a := range strings.Split(allow, ",") {
        if strings.EqualFold(strings.TrimSpace(a), host) { return true }
    }
    return false
}

func parseCIDRs(csv string) []*net.IPNet {
    var out []*net.IPNet
    for _, s := range strings.Split(csv, ",") {
        s = strings.TrimSpace(s)
        if s == "" { continue }
        if _, n, err := net.ParseCIDR(s); err == nil { out = append(out, n) }
    }
    return out
}

func cidrAllowed(cidr string, allows []*net.IPNet) bool {
    _, cand, err := net.ParseCIDR(cidr)
    if err != nil { return false }
    for _, a := range allows {
        if cidrWithin(cand, a) { return true }
    }
    return false
}

func cidrWithin(child, parent *net.IPNet) bool {
    // parent must contain child's network and broadcast range
    return parent.Contains(child.IP) && parent.Contains(lastIP(child))
}

func lastIP(n *net.IPNet) net.IP {
    ip := n.IP
    mask := n.Mask
    last := make(net.IP, len(ip))
    for i := range ip {
        last[i] = ip[i] | ^mask[i]
    }
    return last
}

func quantityLEQ(val resource.Quantity, max string) bool {
    // compare Kubernetes quantities (Quantity <= max)
    qm, err := resource.ParseQuantity(max)
    if err != nil { return false }
    return val.Cmp(qm) <= 0
}
