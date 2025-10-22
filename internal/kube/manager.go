package kube

import (
	"context"
	"sync"

	authv1 "k8s.io/api/authentication/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/vaheed/kubeOP/kubeop-operator/api/v1alpha1"
)

type Manager struct {
	mu       sync.RWMutex
	clients  map[string]ctrlclient.Client
	restcfgs map[string]*rest.Config
}

func NewManager() *Manager {
	return &Manager{clients: make(map[string]ctrlclient.Client), restcfgs: make(map[string]*rest.Config)}
}

// BuildRESTConfigFromKubeconfig parses kubeconfig bytes into a REST config.
func BuildRESTConfigFromKubeconfig(kubeconfig []byte) (*rest.Config, error) {
	// Directly from bytes
	return clientcmd.RESTConfigFromKubeConfig(kubeconfig)
}

// BuildClientFromREST returns a controller-runtime client with a scheme including core APIs.
func BuildClientFromREST(cfg *rest.Config) (ctrlclient.Client, error) {
	sch := clientgoscheme.Scheme
	// Ensure authv1 registered (for TokenRequest types if needed elsewhere)
	_ = authv1.AddToScheme(sch)
	_ = apiextensionsv1.AddToScheme(sch)
	_ = appv1alpha1.AddToScheme(sch)
	return ctrlclient.New(cfg, ctrlclient.Options{Scheme: sch})
}

// Get returns cached client if present.
func (m *Manager) Get(id string) (ctrlclient.Client, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.clients[id]
	return c, ok
}

// Put stores a client and rest config in cache.
func (m *Manager) Put(id string, c ctrlclient.Client, cfg *rest.Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.clients[id] = c
	m.restcfgs[id] = cfg
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
	rc, err := BuildRESTConfigFromKubeconfig(kubeconfig)
	if err != nil {
		return nil, err
	}
	c, err := BuildClientFromREST(rc)
	if err != nil {
		return nil, err
	}
	m.Put(id, c, rc)
	return c, nil
}

// GetClientset returns a typed clientset for the cluster id, creating if needed.
func (m *Manager) GetClientset(ctx context.Context, id string, loader func(context.Context) ([]byte, error)) (kubernetes.Interface, error) {
	m.mu.RLock()
	rc, ok := m.restcfgs[id]
	m.mu.RUnlock()
	if !ok {
		kubeconfig, err := loader(ctx)
		if err != nil {
			return nil, err
		}
		var err2 error
		rc, err2 = BuildRESTConfigFromKubeconfig(kubeconfig)
		if err2 != nil {
			return nil, err2
		}
		// also build and cache controller-runtime client for consistency
		if c, err := BuildClientFromREST(rc); err == nil {
			m.Put(id, c, rc)
		} else {
			// still store restcfg
			m.mu.Lock()
			m.restcfgs[id] = rc
			m.mu.Unlock()
		}
	}
	return kubernetes.NewForConfig(rc)
}
