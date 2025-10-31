package controllers

import (
    "context"
    "fmt"
    "os"
    "strconv"
    "crypto/sha1"
    "encoding/hex"
    "net/http"
    "time"

    appsv1 "k8s.io/api/apps/v1"
    batchv1 "k8s.io/api/batch/v1"
    corev1 "k8s.io/api/core/v1"
    resource "k8s.io/apimachinery/pkg/api/resource"
    networkingv1 "k8s.io/api/networking/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
    "k8s.io/apimachinery/pkg/types"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/controller"
    "sigs.k8s.io/controller-runtime/pkg/log"

    v1alpha1 "github.com/vaheed/kubeop/internal/operator/apis/paas/v1alpha1"
)

func setCondition(conds *[]v1alpha1.Condition, t, status, reason, msg string) {
    now := metav1.NewTime(time.Now())
    c := v1alpha1.Condition{Type: t, Status: status, Reason: reason, Message: msg, LastTransitionTime: now}
    // replace existing of same type
    found := false
    for i := range *conds {
        if (*conds)[i].Type == t {
            (*conds)[i] = c
            found = true
            break
        }
    }
    if !found {
        *conds = append(*conds, c)
    }
}

// Tenant reconciler: no-op beyond setting Ready for now.
type TenantReconciler struct{ client.Client }

func (r *TenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    lg := log.FromContext(ctx)
    var t v1alpha1.Tenant
    if err := r.Get(ctx, req.NamespacedName, &t); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    setCondition(&t.Status.Conditions, "Ready", "True", "Bootstrapped", "Tenant initialized")
    t.Status.Ready = true
    if err := r.Status().Update(ctx, &t); err != nil {
        lg.Error(err, "update tenant status")
        return ctrl.Result{}, err
    }
    return ctrl.Result{}, nil
}
func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&v1alpha1.Tenant{}).
        WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
        Complete(r)
}

// Project reconciler: ensure namespace exists and set ready.
type ProjectReconciler struct{ client.Client }

func (r *ProjectReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    lg := log.FromContext(ctx)
    var p v1alpha1.Project
    if err := r.Get(ctx, req.NamespacedName, &p); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    nsName := fmt.Sprintf("kubeop-%s-%s", p.Spec.TenantRef, p.Spec.Name)
    var ns corev1.Namespace
    if err := r.Get(ctx, types.NamespacedName{Name: nsName}, &ns); err != nil {
        if apierrors.IsNotFound(err) {
            ns = corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName, Labels: map[string]string{
                "app.kubeop.io/tenant": p.Spec.TenantRef,
                "app.kubeop.io/project": p.Spec.Name,
            }}}
            if err := r.Create(ctx, &ns); err != nil {
                lg.Error(err, "create namespace")
                setCondition(&p.Status.Conditions, "Ready", "False", "CreateFailed", err.Error())
                _ = r.Status().Update(ctx, &p)
                return ctrl.Result{}, err
            }
        } else {
            return ctrl.Result{}, err
        }
    }
    // ensure baseline policies
    if err := r.ensureLimitRange(ctx, nsName); err != nil { return ctrl.Result{}, err }
    if err := r.ensureResourceQuota(ctx, nsName); err != nil { return ctrl.Result{}, err }
    if err := r.ensureEgressPolicy(ctx, nsName); err != nil { return ctrl.Result{}, err }
    if err := r.ensureIngressIsolation(ctx, nsName); err != nil { return ctrl.Result{}, err }

    p.Status.Namespace = nsName
    setCondition(&p.Status.Conditions, "Ready", "True", "Bootstrapped", "Project namespace ready")
    p.Status.Ready = true
    if err := r.Status().Update(ctx, &p); err != nil {
        lg.Error(err, "update project status")
        return ctrl.Result{}, err
    }
    return ctrl.Result{}, nil
}
func (r *ProjectReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&v1alpha1.Project{}).
        Owns(&corev1.Namespace{}).
        WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
        Complete(r)
}

func (r *ProjectReconciler) ensureLimitRange(ctx context.Context, ns string) error {
    var lr corev1.LimitRange
    err := r.Get(ctx, types.NamespacedName{Namespace: ns, Name: "kubeop-defaults"}, &lr)
    if apierrors.IsNotFound(err) {
        lr = corev1.LimitRange{ObjectMeta: metav1.ObjectMeta{Name: "kubeop-defaults", Namespace: ns}, Spec: corev1.LimitRangeSpec{Limits: []corev1.LimitRangeItem{{
            Type: corev1.LimitTypeContainer,
            DefaultRequest: corev1.ResourceList{corev1.ResourceCPU: resourceMust("100m"), corev1.ResourceMemory: resourceMust("64Mi")},
            Default:        corev1.ResourceList{corev1.ResourceCPU: resourceMust("500m"), corev1.ResourceMemory: resourceMust("256Mi")},
        }}}}
        return r.Create(ctx, &lr)
    }
    return err
}

func (r *ProjectReconciler) ensureResourceQuota(ctx context.Context, ns string) error {
    var rq corev1.ResourceQuota
    err := r.Get(ctx, types.NamespacedName{Namespace: ns, Name: "kubeop-quota"}, &rq)
    if apierrors.IsNotFound(err) {
        rq = corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Name: "kubeop-quota", Namespace: ns}, Spec: corev1.ResourceQuotaSpec{Hard: corev1.ResourceList{
            corev1.ResourcePods:           resourceMust("10"),
            corev1.ResourceRequestsCPU:    resourceMust("1"),
            corev1.ResourceRequestsMemory: resourceMust("1Gi"),
        }}}
        return r.Create(ctx, &rq)
    }
    return err
}

func (r *ProjectReconciler) ensureEgressPolicy(ctx context.Context, ns string) error {
    var np networkingv1.NetworkPolicy
    err := r.Get(ctx, types.NamespacedName{Namespace: ns, Name: "kubeop-egress"}, &np)
    if apierrors.IsNotFound(err) {
        np = networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "kubeop-egress", Namespace: ns}, Spec: networkingv1.NetworkPolicySpec{
            PodSelector: metav1.LabelSelector{},
            PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
            Egress: []networkingv1.NetworkPolicyEgressRule{{}},
        }}
        return r.Create(ctx, &np)
    }
    return err
}

// ensureIngressIsolation creates a NetworkPolicy that allows ingress only from pods
// within the same namespace, effectively blocking cross-namespace traffic.
func (r *ProjectReconciler) ensureIngressIsolation(ctx context.Context, ns string) error {
    var np networkingv1.NetworkPolicy
    err := r.Get(ctx, types.NamespacedName{Namespace: ns, Name: "kubeop-ingress"}, &np)
    if apierrors.IsNotFound(err) {
        allowSameNS := networkingv1.NetworkPolicyPeer{PodSelector: &metav1.LabelSelector{}}
        np = networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "kubeop-ingress", Namespace: ns}, Spec: networkingv1.NetworkPolicySpec{
            PodSelector: metav1.LabelSelector{},
            PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
            Ingress: []networkingv1.NetworkPolicyIngressRule{{From: []networkingv1.NetworkPolicyPeer{allowSameNS}}},
        }}
        return r.Create(ctx, &np)
    }
    return err
}

func resourceMust(s string) resource.Quantity { q := resource.MustParse(s); return q }

// App reconciler: set a revision and ready.
type AppReconciler struct{ client.Client }

func (r *AppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    lg := log.FromContext(ctx)
    var a v1alpha1.App
    if err := r.Get(ctx, req.NamespacedName, &a); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    // Optional CPU spin for load testing (e2e): burn CPU for configured milliseconds per reconcile
    if msStr := os.Getenv("KUBEOP_RECONCILE_SPIN_MS"); msStr != "" {
        if ms, err := strconv.Atoi(msStr); err == nil && ms > 0 {
            t0 := time.Now()
            for time.Since(t0) < time.Duration(ms)*time.Millisecond {
            }
        }
    }
    // ensure deployment for Image type
    if a.Spec.Type == "Image" && a.Spec.Image != "" {
        depName := "app-" + a.Name
        var dep appsv1.Deployment
        err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: depName}, &dep)
        replicas := int32(1)
        labels := map[string]string{"app.kubeop.io/app": a.Name}
        if apierrors.IsNotFound(err) {
            dep = appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: depName, Namespace: req.Namespace, Labels: labels, Annotations: map[string]string{"kubeop.io/revision": computeImageRev(a.Spec.Image)}}, Spec: appsv1.DeploymentSpec{
                Replicas: &replicas,
                Selector: &metav1.LabelSelector{MatchLabels: labels},
                Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: labels, Annotations: map[string]string{"kubeop.io/revision": computeImageRev(a.Spec.Image)}}, Spec: corev1.PodSpec{
                    SecurityContext: &corev1.PodSecurityContext{RunAsNonRoot: boolPtr(true), SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}},
                    Containers: []corev1.Container{{
                        Name:  "app",
                        Image: a.Spec.Image,
                        Ports: []corev1.ContainerPort{{ContainerPort: 80}},
                        SecurityContext: &corev1.SecurityContext{
                            AllowPrivilegeEscalation: boolPtr(false),
                            ReadOnlyRootFilesystem:   boolPtr(true),
                            RunAsNonRoot:             boolPtr(true),
                            Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
                        },
                    }},
                }},
            }}
            if err := r.Create(ctx, &dep); err != nil { return ctrl.Result{}, err }
        } else if err == nil {
            if len(dep.Spec.Template.Spec.Containers) == 0 {
                dep.Spec.Template.Spec.Containers = []corev1.Container{{
                    Name:  "app",
                    Image: a.Spec.Image,
                    SecurityContext: &corev1.SecurityContext{
                        AllowPrivilegeEscalation: boolPtr(false),
                        ReadOnlyRootFilesystem:   boolPtr(true),
                        RunAsNonRoot:             boolPtr(true),
                        Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
                    },
                }}
            } else {
                dep.Spec.Template.Spec.Containers[0].Image = a.Spec.Image
                // enforce baseline security context on updates
                if dep.Spec.Template.Spec.Containers[0].SecurityContext == nil {
                    dep.Spec.Template.Spec.Containers[0].SecurityContext = &corev1.SecurityContext{}
                }
                sc := dep.Spec.Template.Spec.Containers[0].SecurityContext
                sc.AllowPrivilegeEscalation = boolPtr(false)
                sc.ReadOnlyRootFilesystem = boolPtr(true)
                sc.RunAsNonRoot = boolPtr(true)
                if sc.Capabilities == nil { sc.Capabilities = &corev1.Capabilities{} }
                sc.Capabilities.Drop = []corev1.Capability{"ALL"}
            }
            if dep.Spec.Template.Spec.SecurityContext == nil {
                dep.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{}
            }
            dep.Spec.Template.Spec.SecurityContext.RunAsNonRoot = boolPtr(true)
            if dep.Spec.Template.Spec.SecurityContext.SeccompProfile == nil {
                dep.Spec.Template.Spec.SecurityContext.SeccompProfile = &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}
            }
            if dep.Spec.Template.Annotations == nil { dep.Spec.Template.Annotations = map[string]string{} }
            dep.Spec.Template.Annotations["kubeop.io/revision"] = computeImageRev(a.Spec.Image)
            if err := r.Update(ctx, &dep); err != nil { return ctrl.Result{}, err }
        } else {
            return ctrl.Result{}, err
        }
    }
    // set revision based on image hash for Image type
    if a.Spec.Type == "Image" && a.Spec.Image != "" {
        a.Status.Revision = computeImageRev(a.Spec.Image)
    } else if a.Status.Revision == "" {
        a.Status.Revision = time.Now().UTC().Format("20060102-150405")
    }
    // reflect deployment readiness
    ready := true
    if a.Spec.Type == "Image" && a.Spec.Image != "" {
        var dep appsv1.Deployment
        if err := r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: "app-" + a.Name}, &dep); err == nil {
            if dep.Status.AvailableReplicas < 1 { ready = false }
        }
    }
    if ready {
        setCondition(&a.Status.Conditions, "Ready", "True", "Converged", "App reconciled")
        a.Status.Ready = true
    } else {
        setCondition(&a.Status.Conditions, "Ready", "False", "Progressing", "Waiting for rollout")
        a.Status.Ready = false
        _ = r.Status().Update(ctx, &a)
        return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
    }
    if err := r.Status().Update(ctx, &a); err != nil {
        lg.Error(err, "update app status")
        return ctrl.Result{}, err
    }
    return ctrl.Result{}, nil
}

func computeImageRev(img string) string {
    h := sha1.New()
    h.Write([]byte(img))
    return hex.EncodeToString(h.Sum(nil))[:12]
}
func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&v1alpha1.App{}).
        WithOptions(controller.Options{MaxConcurrentReconciles: 2}).
        Complete(r)
}

// DNSRecord reconciler: mock provider success.
type DNSRecordReconciler struct{
    client.Client
    Endpoint string
}

func (r *DNSRecordReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    var d v1alpha1.DNSRecord
    if err := r.Get(ctx, req.NamespacedName, &d); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    if r.Endpoint != "" {
        // fire-and-forget POST
        _ , _ = http.Post(r.Endpoint+"/v1/dnsrecords", "application/json", http.NoBody)
    }
    d.Status.Ready = true
    d.Status.Message = "mocked"
    if err := r.Status().Update(ctx, &d); err != nil {
        return ctrl.Result{}, err
    }
    return ctrl.Result{}, nil
}
func (r *DNSRecordReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&v1alpha1.DNSRecord{}).
        Complete(r)
}

// Certificate reconciler: set ready immediately.
type CertificateReconciler struct{
    client.Client
    Endpoint string
}

func (r *CertificateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    var c v1alpha1.Certificate
    if err := r.Get(ctx, req.NamespacedName, &c); err != nil {
        return ctrl.Result{}, client.IgnoreNotFound(err)
    }
    if r.Endpoint != "" {
        _, _ = http.Post(r.Endpoint+"/v1/certificates", "application/json", http.NoBody)
    }
    c.Status.Ready = true
    c.Status.Message = "issued"
    if err := r.Status().Update(ctx, &c); err != nil {
        return ctrl.Result{}, err
    }
    return ctrl.Result{}, nil
}
func (r *CertificateReconciler) SetupWithManager(mgr ctrl.Manager) error {
    return ctrl.NewControllerManagedBy(mgr).
        For(&v1alpha1.Certificate{}).
        Owns(&appsv1.Deployment{}).
        Complete(r)
}

func boolPtr(b bool) *bool { return &b }

// buildHookJob returns a Kubernetes Job to run a single hook container for the given app, revision, and phase.
func buildHookJob(a *v1alpha1.App, hk v1alpha1.Hook, rev, phase string) *batchv1.Job {
    name := fmt.Sprintf("hook-%s-%s-%s", phase, a.Name, rev)
    backoff := int32(0)
    ttl := int32(60)
    job := &batchv1.Job{
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: a.Namespace,
            Labels: map[string]string{
                "app.kubeop.io/app":      a.Name,
                "app.kubeop.io/revision": rev,
                "app.kubeop.io/phase":    phase,
            },
        },
        Spec: batchv1.JobSpec{
            BackoffLimit:            &backoff,
            TTLSecondsAfterFinished: &ttl,
            Template: corev1.PodTemplateSpec{
                ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"job-name": name}},
                Spec: corev1.PodSpec{
                    RestartPolicy: corev1.RestartPolicyNever,
                    Containers: []corev1.Container{{
                        Name:  "hook",
                        Image: hk.Image,
                        Args:  hk.Args,
                    }},
                },
            },
        },
    }
    return job
}
