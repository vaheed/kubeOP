package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	appsv1 "k8s.io/api/apps/v1"
	authv1 "k8s.io/api/authentication/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"kubeop/internal/crypto"
	"kubeop/internal/dns"
	"kubeop/internal/store"
	"kubeop/internal/util"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

// ---------- Flavors ----------

type Flavor struct {
	CPU      string
	Memory   string
	Replicas int32
	PVCSize  string // optional
}

func builtinFlavors() map[string]Flavor {
	return map[string]Flavor{
		"f1-small":  {CPU: "200m", Memory: "256Mi", Replicas: 1},
		"f2-medium": {CPU: "500m", Memory: "512Mi", Replicas: 2},
		"f3-large":  {CPU: "1", Memory: "1Gi", Replicas: 2, PVCSize: "5Gi"},
	}
}

// ---------- Templates ----------

type TemplateCreateInput struct {
	Name string
	Kind string
	Spec map[string]any
}

type TemplateCreateOutput struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
}

func (s *Service) CreateTemplate(ctx context.Context, in TemplateCreateInput) (TemplateCreateOutput, error) {
	if strings.TrimSpace(in.Name) == "" {
		return TemplateCreateOutput{}, errors.New("name is required")
	}
	if in.Kind == "" {
		return TemplateCreateOutput{}, errors.New("kind is required")
	}
	id := uuid.New().String()
	if err := s.st.CreateTemplate(ctx, id, in.Name, in.Kind, in.Spec); err != nil {
		return TemplateCreateOutput{}, err
	}
	return TemplateCreateOutput{ID: id, Name: in.Name, Kind: in.Kind}, nil
}

// ---------- Deploy App ----------

type AppPort struct {
	ContainerPort int32
	ServicePort   int32
	Protocol      string // TCP/UDP
	ServiceType   string // ClusterIP|LoadBalancer
}

type AppDeployInput struct {
	ProjectID     string
	Name          string
	Flavor        string
	Resources     map[string]string
	Replicas      *int32
	Env           map[string]string
	Secrets       []string
	Ports         []AppPort
	Domain        string
	Repo          string
	WebhookSecret string

	Image     string
	Helm      map[string]any
	Manifests []string
}

type AppDeployOutput struct {
	AppID   string `json:"appId"`
	Name    string `json:"name"`
	Service string `json:"service,omitempty"`
	Ingress string `json:"ingress,omitempty"`
}

// -------- App Status / Listing --------

type ServiceSummary struct {
	Name  string  `json:"name"`
	Type  string  `json:"type"`
	Ports []int32 `json:"ports"`
}

type PodSummary struct {
	Name  string `json:"name"`
	Phase string `json:"phase"`
	Ready bool   `json:"ready"`
}

type AppStatus struct {
	AppID        string          `json:"appId"`
	Name         string          `json:"name"`
	Desired      int32           `json:"desired"`
	Ready        int32           `json:"ready"`
	Available    int32           `json:"available"`
	Service      *ServiceSummary `json:"service,omitempty"`
	IngressHosts []string        `json:"ingressHosts,omitempty"`
	Pods         []PodSummary    `json:"pods,omitempty"`
}

// CollectAppStatus queries the Kubernetes API to summarize deployment, service,
// ingress, and pod readiness for an application. The helper centralizes the
// logic shared by list and detail endpoints and logs transient failures at the
// warn level to aid operators without failing the overall request.
func CollectAppStatus(ctx context.Context, c crclient.Client, namespace string, app store.App, logger *slog.Logger) AppStatus {
	if logger == nil {
		logger = slog.Default()
	}
	log := logger.With("appID", app.ID, "namespace", namespace)
	sel := map[string]string{"kubeop.app-id": app.ID}
	st := AppStatus{AppID: app.ID, Name: app.Name}

	dep := &appsv1.Deployment{}
	if err := c.Get(ctx, crclient.ObjectKey{Namespace: namespace, Name: util.Slugify(app.Name)}, dep); err != nil {
		if !apierrors.IsNotFound(err) {
			log.WarnContext(ctx, "failed to fetch deployment status", "error", err)
		}
	} else {
		if dep.Spec.Replicas != nil {
			st.Desired = *dep.Spec.Replicas
		}
		st.Ready = dep.Status.ReadyReplicas
		st.Available = dep.Status.AvailableReplicas
	}

	var svcs corev1.ServiceList
	if err := c.List(ctx, &svcs, crclient.InNamespace(namespace), crclient.MatchingLabels(sel)); err != nil {
		log.WarnContext(ctx, "failed to list services for app", "error", err)
	} else {
		for _, svc := range svcs.Items {
			sum := ServiceSummary{Name: svc.Name, Type: string(svc.Spec.Type)}
			for _, sp := range svc.Spec.Ports {
				sum.Ports = append(sum.Ports, sp.Port)
			}
			st.Service = &sum
			break
		}
	}

	var ings netv1.IngressList
	if err := c.List(ctx, &ings, crclient.InNamespace(namespace), crclient.MatchingLabels(sel)); err != nil {
		log.WarnContext(ctx, "failed to list ingresses for app", "error", err)
	} else {
		for _, ing := range ings.Items {
			for _, rule := range ing.Spec.Rules {
				if rule.Host != "" {
					st.IngressHosts = append(st.IngressHosts, rule.Host)
				}
			}
		}
	}

	var pods corev1.PodList
	if err := c.List(ctx, &pods, crclient.InNamespace(namespace), crclient.MatchingLabels(sel)); err != nil {
		log.WarnContext(ctx, "failed to list pods for app", "error", err)
	} else {
		for _, pod := range pods.Items {
			ready := false
			for _, cs := range pod.Status.ContainerStatuses {
				if cs.Ready {
					ready = true
					break
				}
			}
			st.Pods = append(st.Pods, PodSummary{Name: pod.Name, Phase: string(pod.Status.Phase), Ready: ready})
		}
	}

	return st
}

func (s *Service) ListProjectAppsStatus(ctx context.Context, projectID string) ([]AppStatus, error) {
	// Load project and client
	p, _, _, err := s.st.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
	c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
	if err != nil {
		return nil, err
	}
	apps, err := s.st.ListAppsByProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]AppStatus, 0, len(apps))
	logger := slog.Default().With("projectID", projectID, "clusterID", p.ClusterID)
	for _, a := range apps {
		out = append(out, CollectAppStatus(ctx, c, p.Namespace, a, logger))
	}
	return out, nil
}

func (s *Service) GetAppStatus(ctx context.Context, projectID, appID string) (AppStatus, error) {
	p, _, _, err := s.st.GetProject(ctx, projectID)
	if err != nil {
		return AppStatus{}, err
	}
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
	c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
	if err != nil {
		return AppStatus{}, err
	}
	a, err := s.st.GetApp(ctx, appID)
	if err != nil {
		return AppStatus{}, err
	}
	if a.ProjectID != projectID {
		return AppStatus{}, errors.New("app does not belong to project")
	}
	logger := slog.Default().With("projectID", projectID, "clusterID", p.ClusterID)
	return CollectAppStatus(ctx, c, p.Namespace, a, logger), nil
}

func (s *Service) ScaleApp(ctx context.Context, projectID, appID string, replicas int32) error {
	return s.updateAppDeployment(ctx, projectID, appID, func(dep *appsv1.Deployment) error {
		dep.Spec.Replicas = &replicas
		return nil
	})
}

func (s *Service) UpdateAppImage(ctx context.Context, projectID, appID, image string) error {
	if strings.TrimSpace(image) == "" {
		return errors.New("image required")
	}
	return s.updateAppDeployment(ctx, projectID, appID, func(dep *appsv1.Deployment) error {
		if len(dep.Spec.Template.Spec.Containers) == 0 {
			dep.Spec.Template.Spec.Containers = []corev1.Container{{Name: "app"}}
		}
		dep.Spec.Template.Spec.Containers[0].Image = image
		return nil
	})
}

func (s *Service) RolloutRestartApp(ctx context.Context, projectID, appID string) error {
	return s.updateAppDeployment(ctx, projectID, appID, func(dep *appsv1.Deployment) error {
		if dep.Annotations == nil {
			dep.Annotations = map[string]string{}
		}
		dep.Annotations["kubeop.io/redeploy"] = time.Now().UTC().Format(time.RFC3339)
		return nil
	})
}

// Attachment helpers ---------------------------------------------------------

func normalizeAttachmentKeys(keys []string) []string {
	seen := make(map[string]bool, len(keys))
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}

// AttachConfigMapEnv mutates the container to reference a ConfigMap either via
// envFrom (all keys) or discrete keys. Exported for tests.
func AttachConfigMapEnv(ctn *corev1.Container, name string, keys []string, prefix string) {
	if ctn == nil {
		return
	}
	prefix = strings.TrimSpace(prefix)
	keys = normalizeAttachmentKeys(keys)
	if len(keys) == 0 {
		for _, src := range ctn.EnvFrom {
			if src.ConfigMapRef != nil && src.ConfigMapRef.Name == name {
				return
			}
		}
		ctn.EnvFrom = append(ctn.EnvFrom, corev1.EnvFromSource{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: name}}})
		return
	}
	for _, key := range keys {
		envName := key
		if prefix != "" {
			envName = prefix + key
		}
		attached := false
		for i := range ctn.Env {
			if ctn.Env[i].Name == envName {
				ctn.Env[i].Value = ""
				ctn.Env[i].ValueFrom = &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: name}, Key: key}}
				attached = true
				break
			}
		}
		if !attached {
			ctn.Env = append(ctn.Env, corev1.EnvVar{
				Name:      envName,
				ValueFrom: &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: name}, Key: key}},
			})
		}
	}
}

// DetachConfigMapEnv removes any env/envFrom references to the ConfigMap.
func DetachConfigMapEnv(ctn *corev1.Container, name string) {
	if ctn == nil {
		return
	}
	filteredFrom := make([]corev1.EnvFromSource, 0, len(ctn.EnvFrom))
	for _, src := range ctn.EnvFrom {
		if src.ConfigMapRef != nil && src.ConfigMapRef.Name == name {
			continue
		}
		filteredFrom = append(filteredFrom, src)
	}
	ctn.EnvFrom = filteredFrom
	filteredEnv := make([]corev1.EnvVar, 0, len(ctn.Env))
	for _, ev := range ctn.Env {
		if ev.ValueFrom != nil && ev.ValueFrom.ConfigMapKeyRef != nil && ev.ValueFrom.ConfigMapKeyRef.Name == name {
			continue
		}
		filteredEnv = append(filteredEnv, ev)
	}
	ctn.Env = filteredEnv
}

// AttachSecretEnv mutates the container to reference a Secret either via
// envFrom (all keys) or discrete keys. Exported for tests.
func AttachSecretEnv(ctn *corev1.Container, name string, keys []string, prefix string) {
	if ctn == nil {
		return
	}
	prefix = strings.TrimSpace(prefix)
	keys = normalizeAttachmentKeys(keys)
	if len(keys) == 0 {
		for _, src := range ctn.EnvFrom {
			if src.SecretRef != nil && src.SecretRef.Name == name {
				return
			}
		}
		ctn.EnvFrom = append(ctn.EnvFrom, corev1.EnvFromSource{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: name}}})
		return
	}
	for _, key := range keys {
		envName := key
		if prefix != "" {
			envName = prefix + key
		}
		attached := false
		for i := range ctn.Env {
			if ctn.Env[i].Name == envName {
				ctn.Env[i].Value = ""
				ctn.Env[i].ValueFrom = &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: name}, Key: key}}
				attached = true
				break
			}
		}
		if !attached {
			ctn.Env = append(ctn.Env, corev1.EnvVar{
				Name:      envName,
				ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: name}, Key: key}},
			})
		}
	}
}

// DetachSecretEnv removes any env/envFrom references to the Secret.
func DetachSecretEnv(ctn *corev1.Container, name string) {
	if ctn == nil {
		return
	}
	filteredFrom := make([]corev1.EnvFromSource, 0, len(ctn.EnvFrom))
	for _, src := range ctn.EnvFrom {
		if src.SecretRef != nil && src.SecretRef.Name == name {
			continue
		}
		filteredFrom = append(filteredFrom, src)
	}
	ctn.EnvFrom = filteredFrom
	filteredEnv := make([]corev1.EnvVar, 0, len(ctn.Env))
	for _, ev := range ctn.Env {
		if ev.ValueFrom != nil && ev.ValueFrom.SecretKeyRef != nil && ev.ValueFrom.SecretKeyRef.Name == name {
			continue
		}
		filteredEnv = append(filteredEnv, ev)
	}
	ctn.Env = filteredEnv
}

// AttachConfigMapToApp patches the app's primary container to reference the
// ConfigMap by envFrom or selected keys.
func (s *Service) AttachConfigMapToApp(ctx context.Context, projectID, appID, name string, keys []string, prefix string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("name is required")
	}
	keys = normalizeAttachmentKeys(keys)
	prefix = strings.TrimSpace(prefix)
	err := s.updateAppDeployment(ctx, projectID, appID, func(dep *appsv1.Deployment) error {
		if len(dep.Spec.Template.Spec.Containers) == 0 {
			return errors.New("app has no containers to attach configmap")
		}
		AttachConfigMapEnv(&dep.Spec.Template.Spec.Containers[0], name, keys, prefix)
		return nil
	})
	if err != nil {
		return err
	}
	mode := "envFrom"
	if len(keys) > 0 {
		mode = "keys"
	}
	slog.InfoContext(ctx, "attached configmap to app", "projectID", projectID, "appID", appID, "configMap", name, "mode", mode, "keys", keys, "prefix", prefix)
	return nil
}

// DetachConfigMapFromApp removes ConfigMap references from the app deployment.
func (s *Service) DetachConfigMapFromApp(ctx context.Context, projectID, appID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("name is required")
	}
	err := s.updateAppDeployment(ctx, projectID, appID, func(dep *appsv1.Deployment) error {
		if len(dep.Spec.Template.Spec.Containers) == 0 {
			return errors.New("app has no containers to detach configmap")
		}
		DetachConfigMapEnv(&dep.Spec.Template.Spec.Containers[0], name)
		return nil
	})
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "detached configmap from app", "projectID", projectID, "appID", appID, "configMap", name)
	return nil
}

// AttachSecretToApp patches the deployment to reference a Secret by envFrom or keys.
func (s *Service) AttachSecretToApp(ctx context.Context, projectID, appID, name string, keys []string, prefix string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("name is required")
	}
	keys = normalizeAttachmentKeys(keys)
	prefix = strings.TrimSpace(prefix)
	err := s.updateAppDeployment(ctx, projectID, appID, func(dep *appsv1.Deployment) error {
		if len(dep.Spec.Template.Spec.Containers) == 0 {
			return errors.New("app has no containers to attach secret")
		}
		AttachSecretEnv(&dep.Spec.Template.Spec.Containers[0], name, keys, prefix)
		return nil
	})
	if err != nil {
		return err
	}
	mode := "envFrom"
	if len(keys) > 0 {
		mode = "keys"
	}
	slog.InfoContext(ctx, "attached secret to app", "projectID", projectID, "appID", appID, "secret", name, "mode", mode, "keys", keys, "prefix", prefix)
	return nil
}

// DetachSecretFromApp removes Secret references from the app deployment.
func (s *Service) DetachSecretFromApp(ctx context.Context, projectID, appID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("name is required")
	}
	err := s.updateAppDeployment(ctx, projectID, appID, func(dep *appsv1.Deployment) error {
		if len(dep.Spec.Template.Spec.Containers) == 0 {
			return errors.New("app has no containers to detach secret")
		}
		DetachSecretEnv(&dep.Spec.Template.Spec.Containers[0], name)
		return nil
	})
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "detached secret from app", "projectID", projectID, "appID", appID, "secret", name)
	return nil
}

func (s *Service) updateAppDeployment(ctx context.Context, projectID, appID string, mutate func(*appsv1.Deployment) error) error {
	p, _, _, err := s.st.GetProject(ctx, projectID)
	if err != nil {
		return err
	}
	a, err := s.st.GetApp(ctx, appID)
	if err != nil {
		return err
	}
	if a.ProjectID != projectID {
		return errors.New("app does not belong to project")
	}
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
	c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
	if err != nil {
		return err
	}
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: p.Namespace, Name: util.Slugify(a.Name)}}
	if err := c.Get(ctx, crclient.ObjectKey{Namespace: dep.Namespace, Name: dep.Name}, dep); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	}
	if err := mutate(dep); err != nil {
		return err
	}
	return apply(ctx, c, dep)
}
func (s *Service) DeployApp(ctx context.Context, in AppDeployInput) (AppDeployOutput, error) {
	if strings.TrimSpace(in.ProjectID) == "" || strings.TrimSpace(in.Name) == "" {
		return AppDeployOutput{}, errors.New("projectId and name are required")
	}
	// Only one source
	srcCount := 0
	if in.Image != "" {
		srcCount++
	}
	if len(in.Manifests) > 0 {
		srcCount++
	}
	if in.Helm != nil {
		srcCount++
	}
	if srcCount != 1 {
		return AppDeployOutput{}, errors.New("provide exactly one of image, manifests, or helm")
	}
	appID := uuid.New().String()

	// Load project and cluster clients
	p, qo, _, err := s.st.GetProject(ctx, in.ProjectID)
	if err != nil {
		return AppDeployOutput{}, err
	}
	overrides, err := DecodeQuotaOverrides(qo)
	if err != nil {
		return AppDeployOutput{}, err
	}
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
	c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
	if err != nil {
		return AppDeployOutput{}, err
	}

	// Determine replicas/resources
	replicas := int32(1)
	if in.Replicas != nil {
		replicas = *in.Replicas
	}
	if in.Flavor != "" {
		if f, ok := builtinFlavors()[in.Flavor]; ok {
			if in.Replicas == nil {
				replicas = f.Replicas
			}
			if in.Resources == nil {
				in.Resources = map[string]string{}
			}
			if _, ok := in.Resources["requests.cpu"]; !ok {
				in.Resources["requests.cpu"] = f.CPU
			}
			if _, ok := in.Resources["requests.memory"]; !ok {
				in.Resources["requests.memory"] = f.Memory
			}
			if _, ok := in.Resources["limits.cpu"]; !ok {
				in.Resources["limits.cpu"] = f.CPU
			}
			if _, ok := in.Resources["limits.memory"]; !ok {
				in.Resources["limits.memory"] = f.Memory
			}
		} else {
			return AppDeployOutput{}, fmt.Errorf("unknown flavor %q", in.Flavor)
		}
	}

	// Enforce LB quota (services.loadbalancers) if requested
	lbRequested := 0
	for _, p := range in.Ports {
		if strings.EqualFold(p.ServiceType, "LoadBalancer") {
			lbRequested++
		}
	}
	if lbRequested > 0 {
		// Count existing LB services in the namespace
		var svcs corev1.ServiceList
		if err := c.List(ctx, &svcs, crclient.InNamespace(p.Namespace)); err == nil {
			existing := 0
			for _, s := range svcs.Items {
				if s.Spec.Type == corev1.ServiceTypeLoadBalancer {
					existing++
				}
			}
			// Allow configured max minus existing
			maxLB := s.cfg.MaxLoadBalancersPerProject
			if v, ok := overrides["services.loadbalancers"]; ok {
				if n, err := parseInt(v); err == nil {
					maxLB = n
				}
			}
			if existing+lbRequested > maxLB {
				return AppDeployOutput{}, fmt.Errorf("exceeds services.loadbalancers quota: %d used, %d requested, max %d", existing, lbRequested, maxLB)
			}
		}
	}

	// Deploy by source type
	var svcName, ingName string
	switch {
	case in.Image != "":
		// Deployment
		dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: p.Namespace, Name: util.Slugify(in.Name), Labels: map[string]string{"kubeop.app-id": appID, "app.kubernetes.io/name": util.Slugify(in.Name)}}}
		dep.Spec.Replicas = &replicas
		dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"kubeop.app-id": appID}}
		dep.Spec.Template.ObjectMeta.Labels = map[string]string{"kubeop.app-id": appID, "app.kubernetes.io/name": util.Slugify(in.Name)}
		ctn := corev1.Container{Name: "app", Image: in.Image}
		// secure defaults to satisfy Pod Security Admission "restricted"
		ctn.SecurityContext = DefaultContainerSecurityContextRestricted()
		// resources
		if len(in.Resources) > 0 {
			ctn.Resources.Requests = corev1.ResourceList{}
			ctn.Resources.Limits = corev1.ResourceList{}
			if v := in.Resources["requests.cpu"]; v != "" {
				ctn.Resources.Requests[corev1.ResourceCPU] = resourceMustParse(v)
			}
			if v := in.Resources["requests.memory"]; v != "" {
				ctn.Resources.Requests[corev1.ResourceMemory] = resourceMustParse(v)
			}
			if v := in.Resources["limits.cpu"]; v != "" {
				ctn.Resources.Limits[corev1.ResourceCPU] = resourceMustParse(v)
			}
			if v := in.Resources["limits.memory"]; v != "" {
				ctn.Resources.Limits[corev1.ResourceMemory] = resourceMustParse(v)
			}
		}
		// env
		for k, v := range in.Env {
			ctn.Env = append(ctn.Env, corev1.EnvVar{Name: k, Value: v})
		}
		// ports
		for _, pr := range in.Ports {
			if pr.ContainerPort > 0 {
				ctn.Ports = append(ctn.Ports, corev1.ContainerPort{ContainerPort: pr.ContainerPort, Protocol: corev1.ProtocolTCP})
			}
		}
		dep.Spec.Template.Spec.Containers = []corev1.Container{ctn}
		if err := apply(ctx, c, dep); err != nil {
			return AppDeployOutput{}, err
		}

		// secrets envFrom
		if len(in.Secrets) > 0 {
			// patch pod template with envFrom
			dep.Spec.Template.Spec.Containers[0].EnvFrom = make([]corev1.EnvFromSource, 0, len(in.Secrets))
			for _, sref := range in.Secrets {
				dep.Spec.Template.Spec.Containers[0].EnvFrom = append(dep.Spec.Template.Spec.Containers[0].EnvFrom, corev1.EnvFromSource{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: sref}}})
			}
			if err := apply(ctx, c, dep); err != nil {
				return AppDeployOutput{}, err
			}
		}

		// Service
		if len(in.Ports) > 0 {
			svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: p.Namespace, Name: dep.Name, Labels: map[string]string{"kubeop.app-id": appID}}}
			// annotations per LB driver
			svc.Annotations = s.lbServiceAnnotations()
			for _, pr := range in.Ports {
				if pr.ServicePort <= 0 {
					continue
				}
				pt := corev1.ServicePort{Port: pr.ServicePort, TargetPort: intstr.FromInt(int(pr.ContainerPort))}
				if strings.EqualFold(pr.Protocol, "UDP") {
					proto := corev1.ProtocolUDP
					pt.Protocol = proto
				}
				svc.Spec.Ports = append(svc.Spec.Ports, pt)
				if strings.EqualFold(pr.ServiceType, "LoadBalancer") {
					svc.Spec.Type = corev1.ServiceTypeLoadBalancer
				}
			}
			if svc.Spec.Type == "" {
				svc.Spec.Type = corev1.ServiceTypeClusterIP
			}
			sel := map[string]string{"kubeop.app-id": appID}
			svc.Spec.Selector = sel
			if err := apply(ctx, c, svc); err != nil {
				return AppDeployOutput{}, err
			}
			svcName = svc.Name
		}

		// Ingress
		host := s.computeIngressHost(in.Domain, p.Namespace, util.Slugify(in.Name))
		if host != "" && len(in.Ports) > 0 {
			httpPort := int32(80)
			for _, pr := range in.Ports {
				if pr.ServicePort == 80 || pr.ServicePort == 8080 {
					httpPort = pr.ServicePort
					break
				}
			}
			ing := &netv1.Ingress{ObjectMeta: metav1.ObjectMeta{Namespace: p.Namespace, Name: dep.Name, Labels: map[string]string{"kubeop.app-id": appID}}}
			pathType := netv1.PathTypePrefix
			ing.Spec.Rules = []netv1.IngressRule{{
				Host: host,
				IngressRuleValue: netv1.IngressRuleValue{HTTP: &netv1.HTTPIngressRuleValue{Paths: []netv1.HTTPIngressPath{{
					Path:     "/",
					PathType: &pathType,
					Backend:  netv1.IngressBackend{Service: &netv1.IngressServiceBackend{Name: dep.Name, Port: netv1.ServiceBackendPort{Number: httpPort}}},
				}}}},
			}}
			// TLS via cert-manager
			if s.cfg.EnableCertManager {
				secretName := dep.Name + "-tls"
				ing.Spec.TLS = []netv1.IngressTLS{{Hosts: []string{host}, SecretName: secretName}}
				// Create Certificate as unstructured to avoid extra deps
				cert := &unstructured.Unstructured{}
				cert.SetGroupVersionKind(schema.GroupVersionKind{Group: "cert-manager.io", Version: "v1", Kind: "Certificate"})
				cert.SetNamespace(p.Namespace)
				cert.SetName(dep.Name + "-cert")
				cert.Object["spec"] = map[string]any{
					"dnsNames":   []string{host},
					"secretName": secretName,
				}
				_ = apply(ctx, c, cert)
			}
			if err := apply(ctx, c, ing); err != nil {
				return AppDeployOutput{}, err
			}
			ingName = ing.Name
			// Ensure DNS record if provider configured and Service has an external IP
			_ = s.ensureDNSForService(ctx, p.ClusterID, p.Namespace, svcName, host)
		}

	case len(in.Manifests) > 0:
		// Apply raw manifests into the project namespace
		for _, doc := range in.Manifests {
			if err := s.applyRawManifest(ctx, p.ClusterID, []byte(doc), p.Namespace, map[string]string{"kubeop.app-id": appID}); err != nil {
				return AppDeployOutput{}, err
			}
		}
	case in.Helm != nil:
		// Minimal Helm support: chart should be a direct URL to a .tgz
		chartRef, _ := in.Helm["chart"].(string)
		values, _ := in.Helm["values"].(map[string]any)
		if strings.TrimSpace(chartRef) == "" {
			return AppDeployOutput{}, errors.New("helm.chart is required and must point to a .tgz URL")
		}
		rendered, err := renderHelmChartFromURL(ctx, chartRef, util.Slugify(in.Name), p.Namespace, values)
		if err != nil {
			return AppDeployOutput{}, err
		}
		if err := s.applyRawManifest(ctx, p.ClusterID, []byte(rendered), p.Namespace, map[string]string{"kubeop.app-id": appID}); err != nil {
			return AppDeployOutput{}, err
		}
	}

	if err := s.st.CreateApp(ctx, appID, in.ProjectID, in.Name, "deployed", in.Repo, in.WebhookSecret, map[string]any{"image": in.Image, "ports": in.Ports, "env": in.Env, "helm": in.Helm}); err != nil {
		slog.Warn("store app create failed", slog.String("error", err.Error()))
	}
	return AppDeployOutput{AppID: appID, Name: in.Name, Service: svcName, Ingress: ingName}, nil
}

// computeIngressHost returns domain as-is if provided, else generates from env if enabled.
func (s *Service) computeIngressHost(domain, namespace, app string) string {
	if domain != "" {
		return domain
	}
	if !s.cfg.PaaSWildcardEnabled || s.cfg.PaaSDomain == "" {
		return ""
	}
	return fmt.Sprintf("%s.%s.%s", app, namespace, s.cfg.PaaSDomain)
}

// lbServiceAnnotations returns driver-specific annotations for Services.
func (s *Service) lbServiceAnnotations() map[string]string {
	if strings.EqualFold(s.cfg.LBDriver, "metallb") {
		if s.cfg.LBMetallbPool != "" {
			return map[string]string{"metallb.universe.tf/address-pool": s.cfg.LBMetallbPool}
		}
	}
	return nil
}

// applyRawManifest allows applying a manifest document with namespace/labels injection.
// For now, keep simple: support only a subset by creating common resources is out-of-scope.
func (s *Service) applyRawManifest(ctx context.Context, clusterID string, raw []byte, namespace string, labels map[string]string) error {
	parts := splitYAMLDocs(string(raw))
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, clusterID) }
	c, err := s.km.GetOrCreate(ctx, clusterID, loader)
	if err != nil {
		return err
	}
	for _, doc := range parts {
		if strings.TrimSpace(doc) == "" {
			continue
		}
		// to unstructured
		js, err := yaml.YAMLToJSON([]byte(doc))
		if err != nil {
			return err
		}
		var u unstructured.Unstructured
		if err := u.UnmarshalJSON(js); err != nil {
			return err
		}
		// inject namespace for namespaced kinds if missing
		if isNamespacedKind(u.GetKind()) {
			if u.GetNamespace() == "" {
				u.SetNamespace(namespace)
			}
		}
		// merge labels
		meta := u.GetLabels()
		if meta == nil {
			meta = map[string]string{}
		}
		for k, v := range labels {
			meta[k] = v
		}
		u.SetLabels(meta)
		if err := apply(ctx, c, &u); err != nil {
			return err
		}
	}
	return nil
}

// ---------- Logs ----------

type AppLogsInput struct {
	ProjectID string
	AppID     string
	Container string
	TailLines *int64
	Follow    bool
}

func (s *Service) StreamAppLogs(ctx context.Context, in AppLogsInput) (io.ReadCloser, func(), error) {
	p, _, _, err := s.st.GetProject(ctx, in.ProjectID)
	if err != nil {
		return nil, func() {}, err
	}
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
	cs, err := s.km.GetClientset(ctx, p.ClusterID, loader)
	if err != nil {
		return nil, func() {}, err
	}
	// find latest pod for app-id
	pods, err := cs.CoreV1().Pods(p.Namespace).List(ctx, metav1.ListOptions{LabelSelector: "kubeop.app-id=" + in.AppID})
	if err != nil || len(pods.Items) == 0 {
		return nil, func() {}, errors.New("no pods for app")
	}
	// pick most recent
	latest := pods.Items[0]
	for _, pod := range pods.Items[1:] {
		if pod.CreationTimestamp.Time.After(latest.CreationTimestamp.Time) {
			latest = pod
		}
	}
	opts := &corev1.PodLogOptions{Follow: in.Follow}
	if in.TailLines != nil {
		opts.TailLines = in.TailLines
	}
	if in.Container != "" {
		opts.Container = in.Container
	}
	rc, err := cs.CoreV1().Pods(p.Namespace).GetLogs(latest.Name, opts).Stream(ctx)
	if err != nil {
		return nil, func() {}, err
	}
	closer := func() { _ = rc.Close() }
	return rc, closer, nil
}

// ---------- Kubeconfig renew ----------

type KubeconfigRenewOutput struct {
	KubeconfigB64 string `json:"kubeconfig_b64"`
}

func (s *Service) RenewProjectKubeconfig(ctx context.Context, projectID string) (KubeconfigRenewOutput, error) {
	p, _, _, err := s.st.GetProject(ctx, projectID)
	if err != nil {
		return KubeconfigRenewOutput{}, err
	}
	if s.cfg.ProjectsInUserNamespace {
		return KubeconfigRenewOutput{}, errors.New("renew not applicable in shared user namespace mode; use user bootstrap")
	}
	// Mint new SA token for tenant-sa and update kubeconfig
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
	c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
	if err != nil {
		return KubeconfigRenewOutput{}, err
	}
	cs, err := s.km.GetClientset(ctx, p.ClusterID, loader)
	if err != nil {
		return KubeconfigRenewOutput{}, err
	}
	saName := "tenant-sa"
	ttl := int64(s.cfg.SATokenTTLSeconds)
	tr := &authv1.TokenRequest{Spec: authv1.TokenRequestSpec{ExpirationSeconds: &ttl}}
	tok, err := cs.CoreV1().ServiceAccounts(p.Namespace).CreateToken(ctx, saName, tr, metav1.CreateOptions{})
	if err != nil {
		return KubeconfigRenewOutput{}, err
	}
	// Rebuild kubeconfig preserving cluster values
	clusterKc, err := s.DecryptClusterKubeconfig(ctx, p.ClusterID)
	if err != nil {
		return KubeconfigRenewOutput{}, err
	}
	kc, err := buildNamespaceScopedKubeconfig(clusterKc, p.Namespace, saName, p.ClusterID, tok.Status.Token)
	if err != nil {
		return KubeconfigRenewOutput{}, err
	}
	enc, err := crypto.EncryptAESGCM([]byte(kc), s.encKey)
	if err != nil {
		return KubeconfigRenewOutput{}, err
	}
	if err := s.st.UpdateProjectKubeconfig(ctx, projectID, enc); err != nil {
		return KubeconfigRenewOutput{}, err
	}
	_ = c // c kept for parity
	return KubeconfigRenewOutput{KubeconfigB64: toB64([]byte(kc))}, nil
}

// ---------- Webhooks ----------

func (s *Service) HandleGitWebhook(ctx context.Context, payload map[string]any, raw []byte, signature string) error {
	// Minimal: try repository.full_name or repository.clone_url, and ref
	repo := getMapString(payload, []string{"repository", "full_name"})
	if repo == "" {
		repo = getMapString(payload, []string{"repository", "clone_url"})
	}
	ref := getString(payload, "ref")
	if repo == "" || ref == "" {
		return errors.New("unsupported payload: missing repository/ref")
	}
	// Find apps by repo (best effort); update annotation to trigger rollout
	apps, err := s.st.FindAppsByRepo(ctx, repo)
	if err != nil {
		return err
	}
	for _, ap := range apps {
		// Verify signature using per-app secret if provided; otherwise fallback to global if set
		secret := ""
		if ap.WebhookSecret.Valid {
			secret = ap.WebhookSecret.String
		} else if s.cfg.GitWebhookSecret != "" {
			secret = s.cfg.GitWebhookSecret
		}
		if secret != "" {
			if !verifyHMAC256(raw, secret, signature) {
				slog.Warn("webhook signature invalid", slog.String("repo", repo), slog.String("app", ap.Name))
				continue
			}
		}
		p, _, _, err := s.st.GetProject(ctx, ap.ProjectID)
		if err != nil {
			continue
		}
		loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
		c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
		if err != nil {
			continue
		}
		// patch deployment annotation
		dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: p.Namespace, Name: util.Slugify(ap.Name)}}
		_ = c.Get(ctx, crclient.ObjectKey{Namespace: dep.Namespace, Name: dep.Name}, dep)
		if dep.Annotations == nil {
			dep.Annotations = map[string]string{}
		}
		dep.Annotations["kubeop.io/redeploy"] = time.Now().UTC().Format(time.RFC3339)
		_ = apply(ctx, c, dep)
	}
	return nil
}

// verifyHMAC256 verifies GitHub-style header: "sha256=<hex>"
func verifyHMAC256(body []byte, secret, header string) bool {
	if len(header) < len("sha256=") || !strings.HasPrefix(header, "sha256=") {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := mac.Sum(nil)
	got, err := hex.DecodeString(header[7:])
	if err != nil {
		return false
	}
	return hmac.Equal(got, want)
}

// Helpers for maps
func getString(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}
func getMapString(m map[string]any, path []string) string {
	cur := any(m)
	for _, p := range path {
		mm, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur, ok = mm[p]
		if !ok {
			return ""
		}
	}
	if s, ok := cur.(string); ok {
		return s
	}
	return ""
}

func parseInt(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// DefaultContainerSecurityContextRestricted returns secure defaults compatible with PSA "restricted".
// These settings assume images can run as non-root and do not require a writable root filesystem.
func DefaultContainerSecurityContextRestricted() *corev1.SecurityContext {
	nonRoot := true
	noPrivEsc := false
	roRoot := true
	prof := corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}
	return &corev1.SecurityContext{
		RunAsNonRoot:             &nonRoot,
		AllowPrivilegeEscalation: &noPrivEsc,
		ReadOnlyRootFilesystem:   &roRoot,
		Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
		SeccompProfile:           &prof,
	}
}

func splitYAMLDocs(s string) []string {
	// simple splitter on lines with only '---'
	lines := strings.Split(s, "\n")
	var docs []string
	var cur []string
	for _, ln := range lines {
		if strings.TrimSpace(ln) == "---" {
			docs = append(docs, strings.Join(cur, "\n"))
			cur = nil
			continue
		}
		cur = append(cur, ln)
	}
	if len(cur) > 0 {
		docs = append(docs, strings.Join(cur, "\n"))
	}
	return docs
}

func isNamespacedKind(kind string) bool {
	// common namespaced kinds
	nsKinds := []string{"Deployment", "StatefulSet", "DaemonSet", "Service", "ConfigMap", "Secret", "Ingress", "Job", "CronJob", "PersistentVolumeClaim", "ServiceAccount", "Role", "RoleBinding"}
	for _, k := range nsKinds {
		if k == kind {
			return true
		}
	}
	return false
}

var (
	helmHostResolverMu sync.RWMutex
	helmHostResolver   = defaultHelmHostResolver

	helmHTTPClientMu sync.RWMutex
	helmHTTPClient   = newDefaultHelmChartHTTPClient()
)

func defaultHelmHostResolver(ctx context.Context, host string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(ctx, "ip", host)
}

// SetHelmChartHostResolver swaps the resolver used to validate chart hosts.
// It returns a restore function to reset the default resolver.
func SetHelmChartHostResolver(resolver func(context.Context, string) ([]net.IP, error)) func() {
	helmHostResolverMu.Lock()
	prev := helmHostResolver
	helmHostResolver = resolver
	helmHostResolverMu.Unlock()

	return func() {
		helmHostResolverMu.Lock()
		helmHostResolver = prev
		helmHostResolverMu.Unlock()
	}
}

func newDefaultHelmChartHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("helm chart download redirect limit exceeded")
			}
			if len(via) == 0 {
				return nil
			}
			allowedHost := via[0].URL.Hostname()
			if !strings.EqualFold(req.URL.Hostname(), allowedHost) {
				return fmt.Errorf("helm chart download redirected to disallowed host %s", req.URL.Hostname())
			}
			return nil
		},
	}
}

// SetHelmChartHTTPClient swaps the client used to download Helm charts.
// It returns a restore function to reset the previous client.
func SetHelmChartHTTPClient(client *http.Client) func() {
	if client == nil {
		client = newDefaultHelmChartHTTPClient()
	}

	helmHTTPClientMu.Lock()
	prev := helmHTTPClient
	helmHTTPClient = client
	helmHTTPClientMu.Unlock()

	return func() {
		helmHTTPClientMu.Lock()
		helmHTTPClient = prev
		helmHTTPClientMu.Unlock()
	}
}

func getHelmChartHTTPClient() *http.Client {
	helmHTTPClientMu.RLock()
	client := helmHTTPClient
	helmHTTPClientMu.RUnlock()
	return client
}

// ValidateHelmChartURL ensures Helm chart downloads only use permitted network targets.
func ValidateHelmChartURL(ctx context.Context, raw string) (*url.URL, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid helm chart url: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, fmt.Errorf("helm chart url must use http or https")
	}
	if parsed.Host == "" {
		return nil, errors.New("helm chart url must include a host")
	}
	if parsed.User != nil {
		return nil, errors.New("helm chart url must not contain credentials")
	}

	host := parsed.Hostname()
	if host == "" {
		return nil, errors.New("helm chart url missing hostname")
	}

	if ip := net.ParseIP(host); ip != nil {
		if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return nil, fmt.Errorf("helm chart url host %s is not allowed", host)
		}
	} else {
		lowered := strings.ToLower(host)
		if lowered == "localhost" {
			return nil, fmt.Errorf("helm chart url host %s is not allowed", host)
		}
		helmHostResolverMu.RLock()
		resolver := helmHostResolver
		helmHostResolverMu.RUnlock()

		ips, err := resolver(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("resolve helm chart host %s: %w", host, err)
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("helm chart url host %s did not resolve to any ip", host)
		}
		for _, ipAddr := range ips {
			addr, ok := netip.AddrFromSlice(ipAddr)
			if !ok {
				return nil, fmt.Errorf("resolve helm chart host %s: invalid ip result", host)
			}
			if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast() {
				return nil, fmt.Errorf("helm chart url host %s resolved to disallowed network %s", host, addr.String())
			}
		}
	}

	return parsed, nil
}

// renderHelmChartFromURL downloads a chart .tgz and renders manifests using provided values.
func renderHelmChartFromURL(ctx context.Context, chartURL, releaseName, namespace string, values map[string]any) (string, error) {
	parsedURL, err := ValidateHelmChartURL(ctx, chartURL)
	if err != nil {
		return "", fmt.Errorf("validate helm chart url: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsedURL.String(), nil)
	if err != nil {
		return "", err
	}
	req.Host = parsedURL.Host

	client := getHelmChartHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download chart failed: %s", resp.Status)
	}
	by, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	// Load chart from archive bytes
	ch, err := loader.LoadArchive(bytes.NewReader(by))
	if err != nil {
		return "", err
	}
	// Prepare values
	if values == nil {
		values = map[string]any{}
	}
	// Render
	vals, err := chartutil.ToRenderValues(ch, values, chartutil.ReleaseOptions{
		Name: releaseName, Namespace: namespace, IsInstall: true, IsUpgrade: false,
	}, chartutil.DefaultCapabilities)
	if err != nil {
		return "", err
	}
	rendered, err := engine.Render(ch, vals)
	if err != nil {
		return "", err
	}
	// Concatenate sorted files for stability
	keys := make([]string, 0, len(rendered))
	for k := range rendered {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out bytes.Buffer
	for i, k := range keys {
		if i > 0 {
			out.WriteString("\n---\n")
		}
		out.WriteString(rendered[k])
	}
	return out.String(), nil
}

// RenderHelmChartFromURLForTest exposes the Helm renderer for integration tests.
func RenderHelmChartFromURLForTest(ctx context.Context, chartURL, releaseName, namespace string, values map[string]any) (string, error) {
	return renderHelmChartFromURL(ctx, chartURL, releaseName, namespace, values)
}

// ensureDNSForService finds the LB IP for a Service and calls DNS provider to upsert host -> IP.
func (s *Service) ensureDNSForService(ctx context.Context, clusterID, namespace, serviceName, host string) error {
	prov := dns.NewProvider(s.cfg)
	if prov == nil || host == "" {
		return nil
	}
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, clusterID) }
	cs, err := s.km.GetClientset(ctx, clusterID, loader)
	if err != nil {
		return err
	}
	svc, err := cs.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return nil
	}
	if len(svc.Status.LoadBalancer.Ingress) == 0 {
		return nil
	}
	ip := svc.Status.LoadBalancer.Ingress[0].IP
	if ip == "" {
		return nil
	}
	return prov.EnsureARecord(host, ip, s.cfg.ExternalDNSTTL)
}

// DeleteApp deletes app resources in Kubernetes (by label) and soft-deletes the app row in DB.
func (s *Service) DeleteApp(ctx context.Context, projectID, appID string) error {
	// Load project to know namespace and cluster
	p, _, _, err := s.st.GetProject(ctx, projectID)
	if err != nil {
		return err
	}
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
	c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
	if err != nil {
		return err
	}
	// label selector
	sel := map[string]string{"kubeop.app-id": appID}
	// Deployments
	{
		var ls appsv1.DeploymentList
		if err := c.List(ctx, &ls, crclient.InNamespace(p.Namespace), crclient.MatchingLabels(sel)); err == nil {
			for i := range ls.Items {
				_ = c.Delete(ctx, &ls.Items[i])
			}
		}
	}
	// StatefulSets
	{
		var ls appsv1.StatefulSetList
		if err := c.List(ctx, &ls, crclient.InNamespace(p.Namespace), crclient.MatchingLabels(sel)); err == nil {
			for i := range ls.Items {
				_ = c.Delete(ctx, &ls.Items[i])
			}
		}
	}
	// DaemonSets
	{
		var ls appsv1.DaemonSetList
		if err := c.List(ctx, &ls, crclient.InNamespace(p.Namespace), crclient.MatchingLabels(sel)); err == nil {
			for i := range ls.Items {
				_ = c.Delete(ctx, &ls.Items[i])
			}
		}
	}
	// Services
	{
		var ls corev1.ServiceList
		if err := c.List(ctx, &ls, crclient.InNamespace(p.Namespace), crclient.MatchingLabels(sel)); err == nil {
			for i := range ls.Items {
				_ = c.Delete(ctx, &ls.Items[i])
			}
		}
	}
	// Ingresses (collect hosts for DNS delete)
	var ingHosts []string
	{
		var ls netv1.IngressList
		if err := c.List(ctx, &ls, crclient.InNamespace(p.Namespace), crclient.MatchingLabels(sel)); err == nil {
			for i := range ls.Items {
				for _, r := range ls.Items[i].Spec.Rules {
					if r.Host != "" {
						ingHosts = append(ingHosts, r.Host)
					}
				}
				_ = c.Delete(ctx, &ls.Items[i])
			}
		}
	}
	// Jobs/CronJobs
	{
		var jls batchv1.JobList
		if err := c.List(ctx, &jls, crclient.InNamespace(p.Namespace), crclient.MatchingLabels(sel)); err == nil {
			for i := range jls.Items {
				_ = c.Delete(ctx, &jls.Items[i])
			}
		}
		var cls batchv1.CronJobList
		if err := c.List(ctx, &cls, crclient.InNamespace(p.Namespace), crclient.MatchingLabels(sel)); err == nil {
			for i := range cls.Items {
				_ = c.Delete(ctx, &cls.Items[i])
			}
		}
	}
	// ConfigMaps/Secrets/PVCs
	{
		var cms corev1.ConfigMapList
		if err := c.List(ctx, &cms, crclient.InNamespace(p.Namespace), crclient.MatchingLabels(sel)); err == nil {
			for i := range cms.Items {
				_ = c.Delete(ctx, &cms.Items[i])
			}
		}
		var secs corev1.SecretList
		if err := c.List(ctx, &secs, crclient.InNamespace(p.Namespace), crclient.MatchingLabels(sel)); err == nil {
			for i := range secs.Items {
				_ = c.Delete(ctx, &secs.Items[i])
			}
		}
		var pvcs corev1.PersistentVolumeClaimList
		if err := c.List(ctx, &pvcs, crclient.InNamespace(p.Namespace), crclient.MatchingLabels(sel)); err == nil {
			for i := range pvcs.Items {
				_ = c.Delete(ctx, &pvcs.Items[i])
			}
		}
	}
	// DNS cleanup
	if len(ingHosts) > 0 {
		prov := dns.NewProvider(s.cfg)
		if prov != nil {
			for _, h := range ingHosts {
				_ = prov.DeleteARecord(h)
			}
		}
	}
	// Soft-delete in DB (ignore missing)
	_ = s.st.SoftDeleteApp(ctx, appID)
	return nil
}
