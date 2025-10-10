package kube

import (
    "context"
    "sync"

    "k8s.io/client-go/tools/clientcmd"
    ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/client/config"
)

type Manager struct {
    mu      sync.RWMutex
    clients map[string]ctrlclient.Client
}

func NewManager() *Manager {
    return &Manager{clients: make(map[string]ctrlclient.Client)}
}

// BuildClientFromKubeconfig creates a controller-runtime client from kubeconfig bytes.
func BuildClientFromKubeconfig(kubeconfig []byte) (ctrlclient.Client, error) {
    loading := &clientcmd.ClientConfigLoadingRules{ExplicitPath: ""}
    overrides := &clientcmd.ConfigOverrides{}
    clientCfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loading, overrides).ClientConfig()
    if err != nil {
        // If explicit path failed (none), fall back to bytes parsing
        cfg, err2 := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
        if err2 != nil {
            return nil, err
        }
        return ctrlclient.New(cfg, ctrlclient.Options{})
    }
    return ctrlclient.New(clientCfg, ctrlclient.Options{})
}

// FromInCluster returns a client using in-cluster config (not used in this project, but handy).
func FromInCluster() (ctrlclient.Client, error) {
    cfg, err := config.GetConfig()
    if err != nil {
        return nil, err
    }
    return ctrlclient.New(cfg, ctrlclient.Options{})
}

// Get returns cached client if present.
func (m *Manager) Get(id string) (ctrlclient.Client, bool) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    c, ok := m.clients[id]
    return c, ok
}

// Put stores a client in cache.
func (m *Manager) Put(id string, c ctrlclient.Client) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.clients[id] = c
}

// GetOrCreate loads or creates a client via loader.
func (m *Manager) GetOrCreate(ctx context.Context, id string, loader func(context.Context) ([]byte, error)) (ctrlclient.Client, error) {
    if c, ok := m.Get(id); ok {
        return c, nil
    }
    kubeconfig, err := loader(ctx)
    if err != nil {
        return nil, err
    }
    c, err := BuildClientFromKubeconfig(kubeconfig)
    if err != nil {
        return nil, err
    }
    m.Put(id, c)
    return c, nil
}

