package service

import (
	"context"
	"errors"
	"github.com/google/uuid"
	authv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubeop/internal/crypto"
	"kubeop/internal/store"
)

type UserBootstrapInput struct {
	UserID    string
	Name      string
	Email     string
	ClusterID string
}

type UserBootstrapOutput struct {
	User          store.User `json:"user"`
	Namespace     string     `json:"namespace"`
	KubeconfigB64 string     `json:"kubeconfig_b64"`
}

// BootstrapUser creates or resolves a user, provisions a per-user namespace on the cluster
// with defaults and SA/RBAC, stores an encrypted kubeconfig, and returns the kubeconfig (base64).
func (s *Service) BootstrapUser(ctx context.Context, in UserBootstrapInput) (UserBootstrapOutput, error) {
	if in.ClusterID == "" {
		return UserBootstrapOutput{}, errors.New("clusterId is required")
	}
	// Resolve or create user
	var u store.User
	var err error
	if in.UserID != "" {
		u, err = s.st.GetUser(ctx, in.UserID)
		if err != nil {
			return UserBootstrapOutput{}, err
		}
	} else {
		if in.Name == "" || in.Email == "" {
			return UserBootstrapOutput{}, errors.New("either userId or name+email are required")
		}
		// try create; if fails (e.g., duplicate), fetch by email
		nu := store.User{ID: uuid.New().String(), Name: in.Name, Email: in.Email}
		if u, err = s.st.CreateUser(ctx, nu); err != nil {
			if u2, err2 := s.st.GetUserByEmail(ctx, in.Email); err2 == nil {
				u = u2
			} else {
				return UserBootstrapOutput{}, err
			}
		}
	}

	// If userspace exists, return it
	if us, enc, err := s.st.GetUserSpace(ctx, u.ID, in.ClusterID); err == nil {
		kc, err := crypto.DecryptAESGCM(enc, s.encKey)
		if err != nil {
			return UserBootstrapOutput{}, err
		}
		return UserBootstrapOutput{User: u, Namespace: us.Namespace, KubeconfigB64: toB64(kc)}, nil
	}

	// Provision and fetch kubeconfig from DB
	if _, err := s.provisionUserSpace(ctx, u.ID, in.ClusterID); err != nil {
		return UserBootstrapOutput{}, err
	}
	us, enc, err := s.st.GetUserSpace(ctx, u.ID, in.ClusterID)
	if err != nil {
		return UserBootstrapOutput{}, err
	}
	kc, err := crypto.DecryptAESGCM(enc, s.encKey)
	if err != nil {
		return UserBootstrapOutput{}, err
	}
	return UserBootstrapOutput{User: u, Namespace: us.Namespace, KubeconfigB64: toB64(kc)}, nil
}

// DeleteUser performs a soft delete in DB and hard-deletes user namespaces across clusters.
func (s *Service) DeleteUser(ctx context.Context, userID string) error {
	if userID == "" {
		return errors.New("user id required")
	}
	// Delete namespaces for all user spaces
	usList, err := s.st.ListUserSpacesByUser(ctx, userID)
	if err == nil {
		for _, us := range usList {
			loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, us.ClusterID) }
			c, err := s.km.GetOrCreate(ctx, us.ClusterID, loader)
			if err != nil {
				continue
			}
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: us.Namespace}}
			_ = c.Delete(ctx, ns)
		}
	}
	// Soft delete apps and projects for this user
	_ = s.st.SoftDeleteAppsByUser(ctx, userID)
	_ = s.st.SoftDeleteProjectsByUser(ctx, userID)
	// Soft delete user
	return s.st.SoftDeleteUser(ctx, userID)
}

type UserKubeconfigRenewOutput struct {
	KubeconfigB64 string `json:"kubeconfig_b64"`
}

func (s *Service) RenewUserKubeconfig(ctx context.Context, userID, clusterID string) (UserKubeconfigRenewOutput, error) {
	if userID == "" || clusterID == "" {
		return UserKubeconfigRenewOutput{}, errors.New("userId and clusterId are required")
	}
	us, _, err := s.st.GetUserSpace(ctx, userID, clusterID)
	if err != nil {
		return UserKubeconfigRenewOutput{}, err
	}
	// Build clients
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, clusterID) }
	cs, err := s.km.GetClientset(ctx, clusterID, loader)
	if err != nil {
		return UserKubeconfigRenewOutput{}, err
	}
	// Token
	ttl := int64(s.cfg.SATokenTTLSeconds)
	tr := &authv1.TokenRequest{Spec: authv1.TokenRequestSpec{ExpirationSeconds: &ttl}}
	const saName = "user-sa"
	tok, err := cs.CoreV1().ServiceAccounts(us.Namespace).CreateToken(ctx, saName, tr, metav1.CreateOptions{})
	if err != nil {
		return UserKubeconfigRenewOutput{}, err
	}
	// cluster label
	var clusterName string
	if cl, err2 := s.st.ListClusters(ctx); err2 == nil {
		for _, cinfo := range cl {
			if cinfo.ID == clusterID {
				clusterName = cinfo.Name
				break
			}
		}
	}
	if clusterName == "" {
		clusterName = "kubeop-target"
	}
	kubeconfigBytes, err := s.DecryptClusterKubeconfig(ctx, clusterID)
	if err != nil {
		return UserKubeconfigRenewOutput{}, err
	}
	userLabel := s.kubeconfigUserLabel(ctx, userID)
	kc, err := buildNamespaceScopedKubeconfig(kubeconfigBytes, us.Namespace, userLabel, clusterName, tok.Status.Token)
	if err != nil {
		return UserKubeconfigRenewOutput{}, err
	}
	enc, err := crypto.EncryptAESGCM([]byte(kc), s.encKey)
	if err != nil {
		return UserKubeconfigRenewOutput{}, err
	}
	if err := s.st.UpdateUserSpaceKubeconfig(ctx, us.ID, enc); err != nil {
		return UserKubeconfigRenewOutput{}, err
	}
	return UserKubeconfigRenewOutput{KubeconfigB64: toB64([]byte(kc))}, nil
}
