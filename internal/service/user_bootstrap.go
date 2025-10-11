package service

import (
    "context"
    "errors"
    "github.com/google/uuid"
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
        if err != nil { return UserBootstrapOutput{}, err }
    } else {
        if in.Name == "" || in.Email == "" { return UserBootstrapOutput{}, errors.New("either userId or name+email are required") }
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
        if err != nil { return UserBootstrapOutput{}, err }
        return UserBootstrapOutput{User: u, Namespace: us.Namespace, KubeconfigB64: toB64(kc)}, nil
    }

    // Provision and fetch kubeconfig from DB
    if _, err := s.provisionUserSpace(ctx, u.ID, in.ClusterID); err != nil {
        return UserBootstrapOutput{}, err
    }
    us, enc, err := s.st.GetUserSpace(ctx, u.ID, in.ClusterID)
    if err != nil { return UserBootstrapOutput{}, err }
    kc, err := crypto.DecryptAESGCM(enc, s.encKey)
    if err != nil { return UserBootstrapOutput{}, err }
    return UserBootstrapOutput{User: u, Namespace: us.Namespace, KubeconfigB64: toB64(kc)}, nil
}

