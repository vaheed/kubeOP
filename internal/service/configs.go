package service

import (
    "context"
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

// ConfigMaps
func (s *Service) CreateConfigMap(ctx context.Context, projectID, name string, data map[string]string) error {
    p, _, _, err := s.st.GetProject(ctx, projectID)
    if err != nil { return err }
    loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
    c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
    if err != nil { return err }
    cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: p.Namespace, Name: name, Labels: map[string]string{"kubeop.project-id": projectID}}}
    cm.Data = data
    return apply(ctx, c, cm)
}

type ConfigEntry struct { Name string `json:"name"` }

func (s *Service) ListConfigMaps(ctx context.Context, projectID string) ([]ConfigEntry, error) {
    p, _, _, err := s.st.GetProject(ctx, projectID)
    if err != nil { return nil, err }
    loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
    c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
    if err != nil { return nil, err }
    var lst corev1.ConfigMapList
    if err := c.List(ctx, &lst, crclient.InNamespace(p.Namespace)); err != nil { return nil, err }
    out := make([]ConfigEntry, 0, len(lst.Items))
    for _, i := range lst.Items { out = append(out, ConfigEntry{Name: i.Name}) }
    return out, nil
}

func (s *Service) DeleteConfigMap(ctx context.Context, projectID, name string) error {
    p, _, _, err := s.st.GetProject(ctx, projectID)
    if err != nil { return err }
    loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
    c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
    if err != nil { return err }
    cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Namespace: p.Namespace, Name: name}}
    return c.Delete(ctx, cm)
}

// Secrets
func (s *Service) CreateSecret(ctx context.Context, projectID, name, typ string, stringData map[string]string) error {
    p, _, _, err := s.st.GetProject(ctx, projectID)
    if err != nil { return err }
    loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
    c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
    if err != nil { return err }
    sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: p.Namespace, Name: name, Labels: map[string]string{"kubeop.project-id": projectID}}}
    if typ != "" { sec.Type = corev1.SecretType(typ) }
    if len(stringData) > 0 { sec.StringData = stringData }
    return apply(ctx, c, sec)
}

type SecretEntry struct { Name string `json:"name"`; Type string `json:"type"` }

func (s *Service) ListSecrets(ctx context.Context, projectID string) ([]SecretEntry, error) {
    p, _, _, err := s.st.GetProject(ctx, projectID)
    if err != nil { return nil, err }
    loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
    c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
    if err != nil { return nil, err }
    var lst corev1.SecretList
    if err := c.List(ctx, &lst, crclient.InNamespace(p.Namespace)); err != nil { return nil, err }
    out := make([]SecretEntry, 0, len(lst.Items))
    for _, i := range lst.Items { out = append(out, SecretEntry{Name: i.Name, Type: string(i.Type)}) }
    return out, nil
}

func (s *Service) DeleteSecret(ctx context.Context, projectID, name string) error {
    p, _, _, err := s.st.GetProject(ctx, projectID)
    if err != nil { return err }
    loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
    c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
    if err != nil { return err }
    sec := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: p.Namespace, Name: name}}
    return c.Delete(ctx, sec)
}

