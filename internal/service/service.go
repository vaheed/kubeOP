package service

import (
    "context"
    "errors"
    "strings"
    "time"

    "github.com/google/uuid"
    "kubeop/internal/config"
    "kubeop/internal/crypto"
    "kubeop/internal/kube"
    "kubeop/internal/store"
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

func (s *Service) CreateUser(ctx context.Context, name, email string) (store.User, error) {
    name = strings.TrimSpace(name)
    email = strings.TrimSpace(strings.ToLower(email))
    if name == "" || email == "" {
        return store.User{}, errors.New("name and email required")
    }
    id := uuid.New().String()
    u := store.User{ID: id, Name: name, Email: email}
    return s.st.CreateUser(ctx, u)
}

func (s *Service) GetUser(ctx context.Context, id string) (store.User, error) {
    return s.st.GetUser(ctx, id)
}

func (s *Service) ListUsers(ctx context.Context, limit, offset int) ([]store.User, error) {
    return s.st.ListUsers(ctx, limit, offset)
}

// DecryptClusterKubeconfig returns the kubeconfig for a given cluster ID.
func (s *Service) DecryptClusterKubeconfig(ctx context.Context, id string) ([]byte, error) {
    b, err := s.st.GetClusterKubeconfigEnc(ctx, id)
    if err != nil {
        return nil, err
    }
    return crypto.DecryptAESGCM(b, s.encKey)
}

