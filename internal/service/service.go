package service

import (
    "context"
    "errors"
    "fmt"
    "encoding/base64"
    "strings"
    "time"

    "github.com/google/uuid"
    "kubeop/internal/config"
    "kubeop/internal/crypto"
    "kubeop/internal/kube"
    "kubeop/internal/store"
    corev1 "k8s.io/api/core/v1"
    rbacv1 "k8s.io/api/rbac/v1"
    netv1 "k8s.io/api/networking/v1"
    authv1 "k8s.io/api/authentication/v1"
    crclient "sigs.k8s.io/controller-runtime/pkg/client"
    apiutil "sigs.k8s.io/controller-runtime/pkg/client/apiutil"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    apierrors "k8s.io/apimachinery/pkg/api/errors"
    "kubeop/internal/util"
    "k8s.io/apimachinery/pkg/api/resource"
    "k8s.io/apimachinery/pkg/util/intstr"
)

type Service struct {
    cfg          *config.Config
    st           *store.Store
    km           *kube.Manager
    encKey       []byte
}

func New(cfg *config.Config, st *store.Store, km *kube.Manager) (*Service, error) {
    if cfg == nil || st == nil {
        return nil, errors.New("missing dependencies")
    }
    key := crypto.DeriveKey(cfg.KcfgEncryptionKey)
    return &Service{cfg: cfg, st: st, km: km, encKey: key}, nil
}

// Health checks DB connectivity.
func (s *Service) Health(ctx context.Context) error {
    return s.st.DB().PingContext(ctx)
}

// RegisterCluster stores a new cluster with encrypted kubeconfig.
func (s *Service) RegisterCluster(ctx context.Context, name, kubeconfig string) (store.Cluster, error) {
    name = strings.TrimSpace(name)
    if name == "" || strings.TrimSpace(kubeconfig) == "" {
        return store.Cluster{}, errors.New("name and kubeconfig required")
    }
    enc, err := crypto.EncryptAESGCM([]byte(kubeconfig), s.encKey)
    if err != nil {
        return store.Cluster{}, err
    }
    id := uuid.New().String()
    c := store.Cluster{ID: id, Name: name, CreatedAt: time.Now().UTC()}
    return s.st.CreateCluster(ctx, c, enc)
}

func (s *Service) ListClusters(ctx context.Context) ([]store.Cluster, error) {
    return s.st.ListClusters(ctx)
}

func (s *Service) GetUser(ctx context.Context, id string) (store.User, error) {
    return s.st.GetUser(ctx, id)
}

func (s *Service) ListUsers(ctx context.Context, limit, offset int) ([]store.User, error) {
    return s.st.ListUsers(ctx, limit, offset)
}

func (s *Service) ListProjects(ctx context.Context, limit, offset int) ([]store.Project, error) {
    return s.st.ListProjects(ctx, limit, offset)
}

func (s *Service) ListUserProjects(ctx context.Context, userID string, limit, offset int) ([]store.Project, error) {
    return s.st.ListProjectsByUser(ctx, userID, limit, offset)
}

// DecryptClusterKubeconfig returns the kubeconfig for a given cluster ID.
func (s *Service) DecryptClusterKubeconfig(ctx context.Context, id string) ([]byte, error) {
    b, err := s.st.GetClusterKubeconfigEnc(ctx, id)
    if err != nil {
        return nil, err
    }
    return crypto.DecryptAESGCM(b, s.encKey)
}

// ClusterHealth summarizes connectivity to a cluster.
type ClusterHealth struct {
    ID      string    `json:"id"`
    Name    string    `json:"name"`
    Healthy bool      `json:"healthy"`
    Error   string    `json:"error,omitempty"`
    Checked time.Time `json:"checked_at"`
}

// CheckCluster attempts a lightweight API call (list namespaces, limit 1).
func (s *Service) CheckCluster(ctx context.Context, id string) (ClusterHealth, error) {
    // Lookup name for response
    var name string
    {
        cs, err := s.st.ListClusters(ctx)
        if err == nil {
            for _, c := range cs {
                if c.ID == id {
                    name = c.Name
                    break
                }
            }
        }
    }
    loader := func(ctx context.Context) ([]byte, error) {
        return s.DecryptClusterKubeconfig(ctx, id)
    }
    c, err := s.km.GetOrCreate(ctx, id, loader)
    if err != nil {
        return ClusterHealth{ID: id, Name: name, Healthy: false, Error: err.Error(), Checked: time.Now().UTC()}, nil
    }
    // simple list with limit 1
    var nl corev1.NamespaceList
    if err := c.List(ctx, &nl, crclient.Limit(1)); err != nil {
        return ClusterHealth{ID: id, Name: name, Healthy: false, Error: err.Error(), Checked: time.Now().UTC()}, nil
    }
    return ClusterHealth{ID: id, Name: name, Healthy: true, Checked: time.Now().UTC()}, nil
}

// CheckAllClusters returns health for all clusters.
func (s *Service) CheckAllClusters(ctx context.Context) ([]ClusterHealth, error) {
    cs, err := s.st.ListClusters(ctx)
    if err != nil {
        return nil, err
    }
    out := make([]ClusterHealth, 0, len(cs))
    for _, c := range cs {
        h, _ := s.CheckCluster(ctx, c.ID) // collect error in status
        if h.Name == "" { h.Name = c.Name }
        out = append(out, h)
    }
    return out, nil
}

// ---------------- Projects / Tenancy ----------------

type ProjectCreateInput struct {
    UserID         string
    UserEmail      string // optional: if UserID empty, create/reuse by email
    UserName       string // optional display name when creating by email
    ClusterID      string
    Name           string
    QuotaOverrides map[string]string // optional resource names -> quantities
}

type ProjectCreateOutput struct {
    Project   store.Project `json:"project"`
    KubeconfigB64 string    `json:"kubeconfig_b64"`
}

func (s *Service) CreateProject(ctx context.Context, in ProjectCreateInput) (ProjectCreateOutput, error) {
    // Resolve/ensure user
    if strings.TrimSpace(in.UserID) == "" {
        email := strings.TrimSpace(strings.ToLower(in.UserEmail))
        if email == "" {
            return ProjectCreateOutput{}, errors.New("either userId or userEmail is required")
        }
        // Try find by email; if missing, create a new user
        u, err := s.st.GetUserByEmail(ctx, email)
        if err != nil {
            name := strings.TrimSpace(in.UserName)
            if name == "" {
                // derive name from email local-part
                if i := strings.Index(email, "@"); i > 0 { name = email[:i] } else { name = email }
            }
            nu := store.User{ID: uuid.New().String(), Name: name, Email: email}
            if nu, err = s.st.CreateUser(ctx, nu); err != nil {
                return ProjectCreateOutput{}, err
            }
            in.UserID = nu.ID
        } else {
            in.UserID = u.ID
        }
    }
    if strings.TrimSpace(in.ClusterID) == "" || strings.TrimSpace(in.Name) == "" {
        return ProjectCreateOutput{}, errors.New("clusterId and name are required")
    }
    // Determine namespace: user's namespace or project-specific
    var nsSlug string
    if s.cfg.ProjectsInUserNamespace {
        us, _, err := s.st.GetUserSpace(ctx, in.UserID, in.ClusterID)
        if err != nil {
            // auto-provision user namespace if missing
            ns, err2 := s.provisionUserSpace(ctx, in.UserID, in.ClusterID)
            if err2 != nil {
                return ProjectCreateOutput{}, fmt.Errorf("failed to provision user space: %w", err2)
            }
            nsSlug = ns
        } else {
            nsSlug = us.Namespace
        }
    } else {
        nsSlug = util.Slugify(fmt.Sprintf("tenant-%s-%s", in.UserID, in.Name))
        if len(nsSlug) > 63 { nsSlug = nsSlug[:63] }
    }

    // Build clients
    loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, in.ClusterID) }
    c, err := s.km.GetOrCreate(ctx, in.ClusterID, loader)
    if err != nil { return ProjectCreateOutput{}, err }
    cs, err := s.km.GetClientset(ctx, in.ClusterID, loader)
    if err != nil { return ProjectCreateOutput{}, err }

    // 1) Namespace with PSA labels (only when creating per-project namespace)
    if !s.cfg.ProjectsInUserNamespace {
        ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsSlug, Labels: map[string]string{}}}
        if s.cfg.PodSecurityLevel != "" {
            if ns.Labels == nil { ns.Labels = map[string]string{} }
            ns.Labels["pod-security.kubernetes.io/enforce"] = s.cfg.PodSecurityLevel
        }
        if err := apply(ctx, c, ns); err != nil { return ProjectCreateOutput{}, err }
    }

    // 2) ResourceQuota (only per-project namespace mode)
    if !s.cfg.ProjectsInUserNamespace {
        rq := &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Name: "tenant-quota", Namespace: nsSlug}}
        rq.Spec.Hard = defaultQuota(s.cfg, in.QuotaOverrides)
        if err := apply(ctx, c, rq); err != nil { return ProjectCreateOutput{}, err }
    }

    // 3) LimitRange (always; name differs if user namespace mode)
    if s.cfg.ProjectsInUserNamespace {
        lr := &corev1.LimitRange{ObjectMeta: metav1.ObjectMeta{Name: "proj-" + util.Slugify(in.Name) + "-limits", Namespace: nsSlug}}
        lr.Spec.Limits = projectLimitRange(s.cfg)
        if err := apply(ctx, c, lr); err != nil { return ProjectCreateOutput{}, err }
    } else {
        lr := &corev1.LimitRange{ObjectMeta: metav1.ObjectMeta{Name: "tenant-limits", Namespace: nsSlug}}
        lr.Spec.Limits = projectLimitRange(s.cfg)
        if err := apply(ctx, c, lr); err != nil { return ProjectCreateOutput{}, err }
    }

    // 4) NetworkPolicies (only in per-project namespace mode)
    if !s.cfg.ProjectsInUserNamespace {
        npDeny := &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "default-deny", Namespace: nsSlug}}
        npDeny.Spec.PodSelector = metav1.LabelSelector{}
        npDeny.Spec.PolicyTypes = []netv1.PolicyType{netv1.PolicyTypeIngress, netv1.PolicyTypeEgress}
        if err := apply(ctx, c, npDeny); err != nil { return ProjectCreateOutput{}, err }

        npDNS := &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-dns", Namespace: nsSlug}}
        npDNS.Spec.PodSelector = metav1.LabelSelector{}
        npDNS.Spec.PolicyTypes = []netv1.PolicyType{netv1.PolicyTypeEgress}
        npDNS.Spec.Egress = []netv1.NetworkPolicyEgressRule{{
            To: []netv1.NetworkPolicyPeer{{
                NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{s.cfg.DNSNamespaceLabelKey: s.cfg.DNSNamespaceLabelValue}},
                PodSelector:       &metav1.LabelSelector{MatchLabels: map[string]string{s.cfg.DNSPodLabelKey: s.cfg.DNSPodLabelValue}},
            }},
            Ports: []netv1.NetworkPolicyPort{{Protocol: protoPtr(corev1.ProtocolUDP), Port: intstrPtr(53)}},
        }}
        if err := apply(ctx, c, npDNS); err != nil { return ProjectCreateOutput{}, err }

        if s.cfg.IngressNamespaceLabelKey != "" && s.cfg.IngressNamespaceLabelValue != "" {
            npIngress := &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-from-ingress", Namespace: nsSlug}}
            npIngress.Spec.PodSelector = metav1.LabelSelector{}
            npIngress.Spec.PolicyTypes = []netv1.PolicyType{netv1.PolicyTypeIngress}
            npIngress.Spec.Ingress = []netv1.NetworkPolicyIngressRule{{
                From: []netv1.NetworkPolicyPeer{{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{s.cfg.IngressNamespaceLabelKey: s.cfg.IngressNamespaceLabelValue}}}},
            }}
            if err := apply(ctx, c, npIngress); err != nil { return ProjectCreateOutput{}, err }
        }
    }

    // 5) ServiceAccount + Role + RoleBinding (only per-project namespace mode)
    var kcStr string
    var enc []byte
    if !s.cfg.ProjectsInUserNamespace {
        sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "tenant-sa", Namespace: nsSlug}}
        if err := apply(ctx, c, sa); err != nil { return ProjectCreateOutput{}, err }
        role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "tenant-role", Namespace: nsSlug}}
        role.Rules = defaultRoleRules()
        if err := apply(ctx, c, role); err != nil { return ProjectCreateOutput{}, err }
        rb := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "tenant-rb", Namespace: nsSlug}}
        rb.Subjects = []rbacv1.Subject{{Kind: "ServiceAccount", Name: "tenant-sa", Namespace: nsSlug}}
        rb.RoleRef = rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: "tenant-role"}
        if err := apply(ctx, c, rb); err != nil { return ProjectCreateOutput{}, err }

        // 6) TokenRequest for SA
        ttl := int64(s.cfg.SATokenTTLSeconds)
        tr := &authv1.TokenRequest{Spec: authv1.TokenRequestSpec{ExpirationSeconds: &ttl}}
        tok, err := cs.CoreV1().ServiceAccounts(nsSlug).CreateToken(ctx, sa.Name, tr, metav1.CreateOptions{})
        if err != nil { return ProjectCreateOutput{}, err }

        // 7) Build kubeconfig (namespace-scoped) using cluster from cluster kubeconfig
        kubeconfigBytes, err := s.DecryptClusterKubeconfig(ctx, in.ClusterID)
        if err != nil { return ProjectCreateOutput{}, err }
        // label cluster name for kubeconfig
        var clusterName string
        if cls, err2 := s.st.ListClusters(ctx); err2 == nil {
            for _, ci := range cls { if ci.ID == in.ClusterID { clusterName = ci.Name; break } }
        }
        if clusterName == "" { clusterName = "kubeop-target" }
        kc, err := buildNamespaceScopedKubeconfig(kubeconfigBytes, nsSlug, sa.Name, clusterName, tok.Status.Token)
        if err != nil { return ProjectCreateOutput{}, err }
        kcStr = kc

        // Store in DB (encrypted)
        e, err := crypto.EncryptAESGCM([]byte(kcStr), s.encKey)
        if err != nil { return ProjectCreateOutput{}, err }
        enc = e
    }
    p := store.Project{ID: uuid.New().String(), UserID: in.UserID, ClusterID: in.ClusterID, Name: in.Name, Namespace: nsSlug}
    var qoJSON []byte
    if len(in.QuotaOverrides) > 0 {
        qoJSON = []byte(mapToJSON(in.QuotaOverrides))
    }
    p, err = s.st.CreateProject(ctx, p, qoJSON, enc)
    if err != nil { return ProjectCreateOutput{}, err }
    if s.cfg.ProjectsInUserNamespace {
        return ProjectCreateOutput{Project: p, KubeconfigB64: ""}, nil
    }
    return ProjectCreateOutput{Project: p, KubeconfigB64: toB64([]byte(kcStr))}, nil
}

// provisionUserSpace ensures a per-user namespace exists on the target cluster and stores
// an encrypted kubeconfig for that namespace in the database.
func (s *Service) provisionUserSpace(ctx context.Context, userID, clusterID string) (string, error) {
    // Build clients
    loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, clusterID) }
    c, err := s.km.GetOrCreate(ctx, clusterID, loader)
    if err != nil { return "", err }
    cs, err := s.km.GetClientset(ctx, clusterID, loader)
    if err != nil { return "", err }

    nsName := "user-" + strings.TrimSpace(userID)
    if nsName == "user-" { return "", errors.New("invalid userID") }
    ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName, Labels: map[string]string{}}}
    if s.cfg.PodSecurityLevel != "" {
        ns.Labels["pod-security.kubernetes.io/enforce"] = s.cfg.PodSecurityLevel
    }
    if err := apply(ctx, c, ns); err != nil { return "", err }

    // Defaults: ResourceQuota and LimitRange
    rq := &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Name: "tenant-quota", Namespace: nsName}}
    rq.Spec.Hard = defaultQuota(s.cfg, nil)
    if err := apply(ctx, c, rq); err != nil { return "", err }
    lr := &corev1.LimitRange{ObjectMeta: metav1.ObjectMeta{Name: "tenant-limits", Namespace: nsName}}
    lr.Spec.Limits = defaultLimitRange(s.cfg)
    if err := apply(ctx, c, lr); err != nil { return "", err }

    // NetworkPolicies: default-deny + allow DNS + allow from ingress namespace
    npDeny := &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "default-deny", Namespace: nsName}}
    npDeny.Spec.PodSelector = metav1.LabelSelector{}
    npDeny.Spec.PolicyTypes = []netv1.PolicyType{netv1.PolicyTypeIngress, netv1.PolicyTypeEgress}
    if err := apply(ctx, c, npDeny); err != nil { return "", err }
    npDNS := &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-dns", Namespace: nsName}}
    npDNS.Spec.PodSelector = metav1.LabelSelector{}
    npDNS.Spec.PolicyTypes = []netv1.PolicyType{netv1.PolicyTypeEgress}
    npDNS.Spec.Egress = []netv1.NetworkPolicyEgressRule{{
        To: []netv1.NetworkPolicyPeer{{
            NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{s.cfg.DNSNamespaceLabelKey: s.cfg.DNSNamespaceLabelValue}},
            PodSelector:       &metav1.LabelSelector{MatchLabels: map[string]string{s.cfg.DNSPodLabelKey: s.cfg.DNSPodLabelValue}},
        }},
        Ports: []netv1.NetworkPolicyPort{{Protocol: protoPtr(corev1.ProtocolUDP), Port: intstrPtr(53)}},
    }}
    if err := apply(ctx, c, npDNS); err != nil { return "", err }
    if s.cfg.IngressNamespaceLabelKey != "" && s.cfg.IngressNamespaceLabelValue != "" {
        npIngress := &netv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "allow-from-ingress", Namespace: nsName}}
        npIngress.Spec.PodSelector = metav1.LabelSelector{}
        npIngress.Spec.PolicyTypes = []netv1.PolicyType{netv1.PolicyTypeIngress}
        npIngress.Spec.Ingress = []netv1.NetworkPolicyIngressRule{{
            From: []netv1.NetworkPolicyPeer{{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{s.cfg.IngressNamespaceLabelKey: s.cfg.IngressNamespaceLabelValue}}}},
        }}
        if err := apply(ctx, c, npIngress); err != nil { return "", err }
    }

    // ServiceAccount + Role/Binding for the user
    sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "user-sa", Namespace: nsName}}
    if err := apply(ctx, c, sa); err != nil { return "", err }
    role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: "user-role", Namespace: nsName}}
    role.Rules = defaultRoleRules()
    if err := apply(ctx, c, role); err != nil { return "", err }
    rb := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: "user-rb", Namespace: nsName}}
    rb.Subjects = []rbacv1.Subject{{Kind: "ServiceAccount", Name: sa.Name, Namespace: nsName}}
    rb.RoleRef = rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: role.Name}
    if err := apply(ctx, c, rb); err != nil { return "", err }

    // Token and kubeconfig
    ttl := int64(s.cfg.SATokenTTLSeconds)
    tr := &authv1.TokenRequest{Spec: authv1.TokenRequestSpec{ExpirationSeconds: &ttl}}
    tok, err := cs.CoreV1().ServiceAccounts(nsName).CreateToken(ctx, sa.Name, tr, metav1.CreateOptions{})
    if err != nil { return "", err }
    // Resolve cluster name for kubeconfig labels
    var clusterName string
    if cl, err := s.st.ListClusters(ctx); err == nil {
        for _, cinfo := range cl { if cinfo.ID == clusterID { clusterName = cinfo.Name; break } }
    }
    if clusterName == "" { clusterName = "kubeop-target" }
    kubeconfigBytes, err := s.DecryptClusterKubeconfig(ctx, clusterID)
    if err != nil { return "", err }
    // Choose a friendly kubeconfig user label for readability. Authentication still uses the ServiceAccount token.
    // Prefer the user's Name, then Email, and finally a stable fallback from userID.
    userLabel := ""
    if u, err := s.st.GetUser(ctx, userID); err == nil {
        if strings.TrimSpace(u.Name) != "" {
            userLabel = SanitizeUserLabel(u.Name)
        } else if strings.TrimSpace(u.Email) != "" {
            userLabel = SanitizeUserLabel(u.Email)
        }
    }
    if userLabel == "" {
        // fallback to a stable label derived from userID
        short := strings.TrimSpace(userID)
        if len(short) > 8 { short = short[:8] }
        userLabel = "user-" + short
    }
    kcStr, err := buildNamespaceScopedKubeconfig(kubeconfigBytes, nsName, userLabel, clusterName, tok.Status.Token)
    if err != nil { return "", err }
    enc, err := crypto.EncryptAESGCM([]byte(kcStr), s.encKey)
    if err != nil { return "", err }

    // Store userspace
    _, err = s.st.CreateUserSpace(ctx, store.UserSpace{ID: uuid.New().String(), UserID: userID, ClusterID: clusterID, Namespace: nsName}, enc)
    if err != nil { return "", err }
    return nsName, nil
}

type ProjectStatus struct {
    Project store.Project `json:"project"`
    Exists  bool          `json:"exists"`
    Details map[string]bool `json:"details"`
}

func (s *Service) GetProjectStatus(ctx context.Context, id string) (ProjectStatus, error) {
    p, _, _, err := s.st.GetProject(ctx, id)
    if err != nil { return ProjectStatus{}, err }
    loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
    c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
    if err != nil { return ProjectStatus{}, err }
    ns := &corev1.Namespace{}
    err = c.Get(ctx, crclient.ObjectKey{Name: p.Namespace}, ns)
    details := map[string]bool{}
    exists := err == nil
    if exists {
        // check key resources
        rq := &corev1.ResourceQuota{}
        details["resourcequota"] = c.Get(ctx, crclient.ObjectKey{Namespace: p.Namespace, Name: "tenant-quota"}, rq) == nil
        lr := &corev1.LimitRange{}
        details["limitrange"] = c.Get(ctx, crclient.ObjectKey{Namespace: p.Namespace, Name: "tenant-limits"}, lr) == nil
        sa := &corev1.ServiceAccount{}
        details["serviceaccount"] = c.Get(ctx, crclient.ObjectKey{Namespace: p.Namespace, Name: "tenant-sa"}, sa) == nil
    }
    return ProjectStatus{Project: p, Exists: exists, Details: details}, nil
}

func (s *Service) SetProjectSuspended(ctx context.Context, id string, suspended bool) error {
    if s.cfg.ProjectsInUserNamespace {
        return errors.New("project suspend/unsuspend not supported when projects share user namespace; use user-level quotas")
    }
    p, qo, _, err := s.st.GetProject(ctx, id)
    if err != nil { return err }
    loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
    c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
    if err != nil { return err }
    // re-apply ResourceQuota
    rq := &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Name: "tenant-quota", Namespace: p.Namespace}}
    if suspended {
        rq.Spec.Hard = corev1.ResourceList{corev1.ResourcePods: resourceMustParse("0")}
    } else {
        // restore defaults + overrides
        overrides := parseJSONToMap(string(qo))
        rq.Spec.Hard = defaultQuota(s.cfg, overrides)
    }
    if err := apply(ctx, c, rq); err != nil { return err }
    return s.st.UpdateProjectSuspended(ctx, id, suspended)
}

func (s *Service) UpdateProjectQuota(ctx context.Context, id string, overrides map[string]string) error {
    if s.cfg.ProjectsInUserNamespace {
        return errors.New("per-project quotas not supported when projects share user namespace; adjust namespace ResourceQuota")
    }
    p, _, _, err := s.st.GetProject(ctx, id)
    if err != nil { return err }
    loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
    c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
    if err != nil { return err }
    rq := &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Name: "tenant-quota", Namespace: p.Namespace}}
    rq.Spec.Hard = defaultQuota(s.cfg, overrides)
    if err := apply(ctx, c, rq); err != nil { return err }
    if len(overrides) > 0 {
        _ = s.st.UpdateProjectQuotaOverrides(ctx, id, []byte(mapToJSON(overrides)))
    }
    return nil
}

func (s *Service) DeleteProject(ctx context.Context, id string) error {
    p, _, _, err := s.st.GetProject(ctx, id)
    if err != nil { return err }
    loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
    c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
    if err != nil { return err }
    if s.cfg.ProjectsInUserNamespace {
        // delete project-specific LimitRange if present
        lr := &corev1.LimitRange{ObjectMeta: metav1.ObjectMeta{Name: "proj-" + util.Slugify(p.Name) + "-limits", Namespace: p.Namespace}}
        _ = c.Delete(ctx, lr) // ignore not found
    } else {
        // delete namespace (cascades)
        ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: p.Namespace}}
        if err := c.Delete(ctx, ns); err != nil && !apierrors.IsNotFound(err) {
            return err
        }
    }
    // Soft-delete apps under this project and the project row
    _ = s.st.SoftDeleteAppsByProject(ctx, id)
    return s.st.SoftDeleteProject(ctx, id)
}

// Helpers
func apply(ctx context.Context, c crclient.Client, obj crclient.Object) error {
    obj.SetManagedFields(nil)
    // Ensure GVK is set for server-side apply
    if obj.GetObjectKind().GroupVersionKind().Empty() {
        if gvk, err := apiutil.GVKForObject(obj, c.Scheme()); err == nil {
            obj.GetObjectKind().SetGroupVersionKind(gvk)
        } else {
            return err
        }
    }
    // Use server-side apply
    return c.Patch(ctx, obj, crclient.Apply, crclient.ForceOwnership, crclient.FieldOwner("kubeop"))
}

func defaultRoleRules() []rbacv1.PolicyRule {
    return []rbacv1.PolicyRule{
        {APIGroups: []string{""}, Resources: []string{"pods","services","configmaps","secrets","persistentvolumeclaims","events"}, Verbs: []string{"get","list","watch","create","update","delete"}},
        // include ReplicaSets so users can inspect Deployment rollouts
        {APIGroups: []string{"apps"}, Resources: []string{"deployments","replicasets","statefulsets","daemonsets"}, Verbs: []string{"get","list","watch","create","update","delete"}},
        {APIGroups: []string{"networking.k8s.io"}, Resources: []string{"ingresses"}, Verbs: []string{"get","list","watch"}},
        {APIGroups: []string{"batch"}, Resources: []string{"jobs","cronjobs"}, Verbs: []string{"get","list","watch","create","update","delete"}},
    }
}

// DefaultUserNamespaceRoleRules exposes the default rules for testing and documentation.
func DefaultUserNamespaceRoleRules() []rbacv1.PolicyRule { return defaultRoleRules() }

func defaultQuota(cfg *config.Config, overrides map[string]string) corev1.ResourceList {
    rl := corev1.ResourceList{}
    // defaults
    rl[corev1.ResourceLimitsMemory] = resourceMustParse(cfg.DefaultQuotaLimitsMemory)
    rl[corev1.ResourceLimitsCPU] = resourceMustParse(cfg.DefaultQuotaLimitsCPU)
    rl[corev1.ResourceLimitsEphemeralStorage] = resourceMustParse(cfg.DefaultQuotaEphemeralStorage)
    rl[corev1.ResourcePods] = resourceMustParse(cfg.DefaultQuotaMaxPods)
    rl[corev1.ResourceRequestsStorage] = resourceMustParse(cfg.DefaultQuotaPVCStorage)
    // LB services via extensions
    // Networking quotas are not standard core resources; document externally.
    for k, v := range overrides {
        rl[corev1.ResourceName(k)] = resourceMustParse(v)
    }
    return rl
}

func defaultLimitRange(cfg *config.Config) []corev1.LimitRangeItem {
    return []corev1.LimitRangeItem{{
        Type: corev1.LimitTypeContainer,
        DefaultRequest: corev1.ResourceList{
            corev1.ResourceCPU:    resourceMustParse(cfg.DefaultLRRequestCPU),
            corev1.ResourceMemory: resourceMustParse(cfg.DefaultLRRequestMemory),
        },
        Default: corev1.ResourceList{
            corev1.ResourceCPU:    resourceMustParse(cfg.DefaultLRLimitCPU),
            corev1.ResourceMemory: resourceMustParse(cfg.DefaultLRLimitMemory),
        },
    }}
}
func projectLimitRange(cfg *config.Config) []corev1.LimitRangeItem {
    return []corev1.LimitRangeItem{{
        Type: corev1.LimitTypeContainer,
        DefaultRequest: corev1.ResourceList{
            corev1.ResourceCPU:    resourceMustParse(cfg.ProjectLRRequestCPU),
            corev1.ResourceMemory: resourceMustParse(cfg.ProjectLRRequestMemory),
        },
        Default: corev1.ResourceList{
            corev1.ResourceCPU:    resourceMustParse(cfg.ProjectLRLimitCPU),
            corev1.ResourceMemory: resourceMustParse(cfg.ProjectLRLimitMemory),
        },
    }}
}

func buildNamespaceScopedKubeconfig(clusterKubeconfig []byte, namespace, userLabel, clusterLabel, token string) (string, error) {
    // For simplicity, we assume context 0
    // In practice, parsing logic should be robust; keeping simple here
    // Use the existing cluster and server/CA from the first entry
    var out strings.Builder
    out.WriteString("apiVersion: v1\nkind: Config\n")
    out.WriteString("clusters:\n")
    out.WriteString("- cluster:\n")
    out.WriteString("    certificate-authority-data: ")
    out.WriteString(extractCABase64(clusterKubeconfig))
    out.WriteString("\n    server: ")
    out.WriteString(extractServer(clusterKubeconfig))
    out.WriteString("\n  name: ")
    out.WriteString(clusterLabel)
    out.WriteString("\n")
    out.WriteString("contexts:\n- context:\n    cluster: ")
    out.WriteString(clusterLabel)
    out.WriteString("\n    namespace: ")
    out.WriteString(namespace)
    out.WriteString("\n    user: ")
    out.WriteString(userLabel)
    out.WriteString("\n  name: ")
    out.WriteString(clusterLabel)
    out.WriteString("\n")
    out.WriteString("current-context: ")
    out.WriteString(clusterLabel)
    out.WriteString("\nusers:\n- name: ")
    out.WriteString(userLabel)
    out.WriteString("\n  user:\n    token: ")
    out.WriteString(token)
    out.WriteString("\n")
    return out.String(), nil
}

// extractServer and extractCABase64 are simple YAML scrapers to keep dependencies light
func extractServer(kc []byte) string {
    s := string(kc)
    key := "server:"
    idx := strings.Index(s, key)
    if idx == -1 { return "" }
    rest := s[idx+len(key):]
    // trim spaces and take first line
    rest = strings.TrimSpace(rest)
    if i := strings.Index(rest, "\n"); i >= 0 { rest = rest[:i] }
    return rest
}
func extractCABase64(kc []byte) string {
    s := string(kc)
    key := "certificate-authority-data:"
    idx := strings.Index(s, key)
    if idx == -1 { return "" }
    rest := s[idx+len(key):]
    rest = strings.TrimSpace(rest)
    if i := strings.Index(rest, "\n"); i >= 0 { rest = rest[:i] }
    return rest
}

func protoPtr(p corev1.Protocol) *corev1.Protocol { return &p }
func intstrPtr(p int32) *intstr.IntOrString { v := intstr.FromInt(int(p)); return &v }
func resourceMustParse(s string) resource.Quantity { q := resource.MustParse(s); return q }
func toB64(b []byte) string { return base64.StdEncoding.EncodeToString(b) }
func mapToJSON(m map[string]string) string {
    var b strings.Builder
    b.WriteString("{")
    first := true
    for k, v := range m {
        if !first { b.WriteString(",") } ; first = false
        b.WriteString("\""); b.WriteString(k); b.WriteString("\":\""); b.WriteString(v); b.WriteString("\"")
    }
    b.WriteString("}")
    return b.String()
}

func parseJSONToMap(s string) map[string]string {
    out := map[string]string{}
    s = strings.TrimSpace(s)
    if s == "" || s == "null" { return out }
    // naive parser for simple string map {"k":"v"}
    // This is intentionally simple to avoid pulling extra deps; for complex cases, store JSONB via callers.
    if s[0] == '{' && s[len(s)-1] == '}' {
        body := s[1:len(s)-1]
        parts := strings.Split(body, ",")
        for _, p := range parts {
            kv := strings.SplitN(p, ":", 2)
            if len(kv) != 2 { continue }
            k := strings.Trim(kv[0], " \"\n\r\t")
            v := strings.Trim(kv[1], " \"\n\r\t")
            if k != "" { out[k] = v }
        }
    }
    return out
}
