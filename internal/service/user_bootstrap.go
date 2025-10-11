package service

import (
    "context"
    "errors"
    "github.com/google/uuid"
    "kubeop/internal/crypto"
    "kubeop/internal/store"
    corev1 "k8s.io/api/core/v1"
    authv1 "k8s.io/api/authentication/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    rbacv1 "k8s.io/api/rbac/v1"
    crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type UserBootstrapInput struct {
    Name      string
    Email     string
    ClusterID string
}

type UserBootstrapOutput struct {
    User          store.User `json:"user"`
    Namespace     string     `json:"namespace"`
    KubeconfigB64 string     `json:"kubeconfig_b64"`
}

// BootstrapUser creates a user (or returns existing) and provisions a namespace, quotas, policies, and a user-scoped kubeconfig.
func (s *Service) BootstrapUser(ctx context.Context, in UserBootstrapInput) (UserBootstrapOutput, error) {
    if in.Name == "" || in.Email == "" || in.ClusterID == "" {
        return UserBootstrapOutput{}, errors.New("name, email, and clusterId are required")
    }
    // create user or reuse existing by email
    u, err := s.st.CreateUser(ctx, store.User{ID: uuid.New().String(), Name: in.Name, Email: in.Email})
    if err != nil {
        // try to fetch by email if already exists
        if u2, err2 := s.st.GetUserByEmail(ctx, in.Email); err2 == nil {
            u = u2
        } else {
            return UserBootstrapOutput{}, err
        }
    }

    // Ensure not already bootstrapped for this cluster
    if us, _, err := s.st.GetUserSpace(ctx, u.ID, in.ClusterID); err == nil && us.Namespace != "" {
        return UserBootstrapOutput{User: u, Namespace: us.Namespace, KubeconfigB64: ""}, nil
    }

    // Build clients
    loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, in.ClusterID) }
    c, err := s.km.GetOrCreate(ctx, in.ClusterID, loader)
    if err != nil { return UserBootstrapOutput{}, err }
    cs, err := s.km.GetClientset(ctx, in.ClusterID, loader)
    if err != nil { return UserBootstrapOutput{}, err }

    // Namespace name
    nsName := "user-" + u.ID
    ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName, Labels: map[string]string{}}}
    if s.cfg.PodSecurityLevel != "" {
        ns.Labels["pod-security.kubernetes.io/enforce"] = s.cfg.PodSecurityLevel
    }
    if err := apply(ctx, c, ns); err != nil { return UserBootstrapOutput{}, err }

    // Defaults: ResourceQuota and LimitRange
    rq := &corev1.ResourceQuota{ObjectMeta: metav1.ObjectMeta{Name: "tenant-quota", Namespace: nsName}}
    rq.Spec.Hard = defaultQuota(s.cfg, nil)
    if err := apply(ctx, c, rq); err != nil { return UserBootstrapOutput{}, err }
    lr := &corev1.LimitRange{ObjectMeta: metav1.ObjectMeta{Name: "tenant-limits", Namespace: nsName}}
    lr.Spec.Limits = defaultLimitRange(s.cfg)
    if err := apply(ctx, c, lr); err != nil { return UserBootstrapOutput{}, err }

    // ServiceAccount + Role/Binding for the user
    sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "user-sa", Namespace: nsName}}
    if err := apply(ctx, c, sa); err != nil { return UserBootstrapOutput{}, err }
    // reuse defaultRoleRules
    if err := ensureRoleBinding(ctx, c, nsName, "user-role", "user-rb", "user-sa"); err != nil { return UserBootstrapOutput{}, err }

    // Token and kubeconfig
    ttl := int64(s.cfg.SATokenTTLSeconds)
    tr := &authv1.TokenRequest{Spec: authv1.TokenRequestSpec{ExpirationSeconds: &ttl}}
    tok, err := cs.CoreV1().ServiceAccounts(nsName).CreateToken(ctx, sa.Name, tr, metav1.CreateOptions{})
    if err != nil { return UserBootstrapOutput{}, err }
    kubeconfigBytes, err := s.DecryptClusterKubeconfig(ctx, in.ClusterID)
    if err != nil { return UserBootstrapOutput{}, err }
    kcStr, err := buildNamespaceScopedKubeconfig(kubeconfigBytes, nsName, sa.Name, tok.Status.Token)
    if err != nil { return UserBootstrapOutput{}, err }
    enc, err := s.encrypt([]byte(kcStr))
    if err != nil { return UserBootstrapOutput{}, err }

    // Store userspace
    us, err := s.st.CreateUserSpace(ctx, store.UserSpace{ID: uuid.New().String(), UserID: u.ID, ClusterID: in.ClusterID, Namespace: nsName}, enc)
    if err != nil { return UserBootstrapOutput{}, err }
    _ = us // not used further yet
    return UserBootstrapOutput{User: u, Namespace: nsName, KubeconfigB64: toB64([]byte(kcStr))}, nil
}

func ensureRoleBinding(ctx context.Context, c crclient.Client, ns, roleName, rbName, saName string) error {
    role := &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Name: roleName, Namespace: ns}}
    role.Rules = defaultRoleRules()
    if err := apply(ctx, c, role); err != nil { return err }
    rb := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: rbName, Namespace: ns}}
    rb.Subjects = []rbacv1.Subject{{Kind: "ServiceAccount", Name: saName, Namespace: ns}}
    rb.RoleRef = rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "Role", Name: roleName}
    return apply(ctx, c, rb)
}

func (s *Service) encrypt(b []byte) ([]byte, error) { return crypto.EncryptAESGCM(b, s.encKey) }
