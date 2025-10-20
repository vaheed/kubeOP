package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/engine"
	"helm.sh/helm/v3/pkg/registry"
	appsv1 "k8s.io/api/apps/v1"
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
	"kubeop/internal/logging"
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

// ---------- Deploy App ----------

type AppPort struct {
	ContainerPort int32  `json:"containerPort"`
	ServicePort   int32  `json:"servicePort"`
	Protocol      string `json:"protocol,omitempty"`    // TCP/UDP
	ServiceType   string `json:"serviceType,omitempty"` // ClusterIP|LoadBalancer
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

// AppValidationOutput summarises the dry-run metadata returned by ValidateApp.
type AppValidationOutput struct {
	ProjectID        string                  `json:"projectId"`
	ProjectNamespace string                  `json:"projectNamespace"`
	ClusterID        string                  `json:"clusterId"`
	Source           string                  `json:"source"`
	Flavor           string                  `json:"flavor,omitempty"`
	KubeName         string                  `json:"kubeName"`
	Replicas         int32                   `json:"replicas"`
	Resources        map[string]string       `json:"resources,omitempty"`
	Ports            []AppPort               `json:"ports,omitempty"`
	Domain           string                  `json:"domain,omitempty"`
	LoadBalancers    LoadBalancerSummary     `json:"loadBalancers"`
	RenderedObjects  []RenderedObjectSummary `json:"renderedObjects,omitempty"`
	HelmChart        string                  `json:"helmChart,omitempty"`
	HelmValues       map[string]any          `json:"helmValues,omitempty"`
	Warnings         []string                `json:"warnings,omitempty"`
}

// LoadBalancerSummary captures the quota state exposed in validation responses.
type LoadBalancerSummary struct {
	Requested int `json:"requested"`
	Existing  int `json:"existing"`
	Limit     int `json:"limit"`
}

// RenderedObjectSummary highlights the kind/name pairs detected during validation renders.
type RenderedObjectSummary struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
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
	Domains      []AppDomainInfo `json:"domains,omitempty"`
}

type AppDomainInfo struct {
	FQDN              string `json:"fqdn"`
	CertificateStatus string `json:"certificateStatus"`
}

// CollectAppStatus queries the Kubernetes API to summarize deployment, service,
// ingress, and pod readiness for an application. The helper centralizes the
// logic shared by list and detail endpoints and logs transient failures at the
// warn level to aid operators without failing the overall request.
func CollectAppStatus(ctx context.Context, c crclient.Client, namespace string, app store.App, logger *zap.Logger) AppStatus {
	if logger == nil {
		logger = logging.L().Named("app-status")
	}
	log := logger.With(zap.String("app_id", app.ID), zap.String("namespace", namespace))
	sel := map[string]string{"kubeop.app-id": app.ID}
	st := AppStatus{AppID: app.ID, Name: app.Name}

	dep := &appsv1.Deployment{}
	if err := c.Get(ctx, crclient.ObjectKey{Namespace: namespace, Name: appKubeName(app)}, dep); err != nil {
		if !apierrors.IsNotFound(err) {
			log.Warn("failed to fetch deployment status", zap.Error(err))
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
		log.Warn("failed to list services for app", zap.Error(err))
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
		log.Warn("failed to list ingresses for app", zap.Error(err))
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
		log.Warn("failed to list pods for app", zap.Error(err))
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

func (s *Service) domainStatuses(ctx context.Context, c crclient.Client, namespace string, app store.App, logger *zap.Logger) []AppDomainInfo {
	domains, err := s.st.ListAppDomains(ctx, app.ID)
	if err != nil {
		if logger != nil {
			logger.Warn("failed to load app domains", zap.Error(err))
		}
		return nil
	}
	if len(domains) == 0 {
		return nil
	}
	infos := make([]AppDomainInfo, 0, len(domains))
	desiredStatus := ""
	if s.cfg.EnableCertManager {
		appSlug := appKubeName(app)
		certName := fmt.Sprintf("%s-cert", appSlug)
		if status, err := s.lookupCertificateStatus(ctx, c, namespace, certName); err != nil {
			if logger != nil {
				logger.Warn("failed to resolve certificate status", zap.Error(err))
			}
		} else {
			desiredStatus = status
		}
	}
	for _, d := range domains {
		status := d.CertStatus
		if desiredStatus != "" {
			status = desiredStatus
		}
		if status == "" {
			status = "pending"
		}
		if status != d.CertStatus {
			if _, err := s.st.UpsertAppDomain(ctx, d.AppID, d.FQDN, status); err != nil && logger != nil {
				logger.Warn("failed to update app domain status", zap.Error(err))
			}
		}
		infos = append(infos, AppDomainInfo{FQDN: d.FQDN, CertificateStatus: status})
	}
	return infos
}

func (s *Service) lookupCertificateStatus(ctx context.Context, c crclient.Client, namespace, certName string) (string, error) {
	cert := &unstructured.Unstructured{}
	cert.SetGroupVersionKind(schema.GroupVersionKind{Group: "cert-manager.io", Version: "v1", Kind: "Certificate"})
	if err := c.Get(ctx, crclient.ObjectKey{Namespace: namespace, Name: certName}, cert); err != nil {
		if apierrors.IsNotFound(err) {
			return "pending", nil
		}
		return "", err
	}
	conditions, found, err := unstructured.NestedSlice(cert.Object, "status", "conditions")
	if err != nil {
		return "", err
	}
	if !found {
		return "pending", nil
	}
	for _, raw := range conditions {
		cond, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if !strings.EqualFold(fmt.Sprint(cond["type"]), "Ready") {
			continue
		}
		status := strings.ToLower(fmt.Sprint(cond["status"]))
		if status == "true" {
			return "issued", nil
		}
		reason := strings.TrimSpace(fmt.Sprint(cond["reason"]))
		message := strings.TrimSpace(fmt.Sprint(cond["message"]))
		if reason == "" && message == "" {
			return "pending", nil
		}
		if reason != "" && message != "" {
			return fmt.Sprintf("error: %s (%s)", strings.ToLower(reason), message), nil
		}
		if reason != "" {
			return fmt.Sprintf("error: %s", strings.ToLower(reason)), nil
		}
		if message != "" {
			return fmt.Sprintf("error: %s", message), nil
		}
	}
	return "pending", nil
}

// DomainStatusesForTest exposes domain status resolution for integration tests.
func DomainStatusesForTest(s *Service, ctx context.Context, c crclient.Client, namespace string, app store.App, logger *zap.Logger) []AppDomainInfo {
	if s == nil {
		return nil
	}
	return s.domainStatuses(ctx, c, namespace, app, logger)
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
	logger := logging.L().With(zap.String("project_id", projectID), zap.String("cluster_id", p.ClusterID))
	for _, a := range apps {
		st := CollectAppStatus(ctx, c, p.Namespace, a, logger)
		st.Domains = s.domainStatuses(ctx, c, p.Namespace, a, logger)
		out = append(out, st)
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
	logger := logging.L().With(zap.String("project_id", projectID), zap.String("cluster_id", p.ClusterID))
	st := CollectAppStatus(ctx, c, p.Namespace, a, logger)
	st.Domains = s.domainStatuses(ctx, c, p.Namespace, a, logger)
	return st, nil
}

func (s *Service) ScaleApp(ctx context.Context, projectID, appID string, replicas int32) error {
	p, a, err := s.updateAppDeployment(ctx, projectID, appID, func(dep *appsv1.Deployment) error {
		dep.Spec.Replicas = &replicas
		return nil
	})
	if err != nil {
		logging.AppErrorLogger(projectID, appID).Error("app_scale_failed", zap.Error(err), zap.Int32("replicas", replicas))
		return err
	}
	fields := []zap.Field{
		zap.String("app_name", a.Name),
		zap.Int32("replicas", replicas),
		zap.String("cluster_id", p.ClusterID),
		zap.String("namespace", p.Namespace),
	}
	logging.ProjectLogger(projectID).Info("app_scaled", fields...)
	if _, err := s.AppendProjectEvent(ctx, EventInput{
		ProjectID: projectID,
		AppID:     appID,
		Kind:      "app_scaled",
		Severity:  SeverityInfo,
		Message:   fmt.Sprintf("scaled app %s", a.Name),
		Meta: map[string]any{
			"replicas":   replicas,
			"cluster_id": p.ClusterID,
			"namespace":  p.Namespace,
			"app_name":   a.Name,
		},
	}); err != nil {
		return err
	}
	logging.AppLogger(projectID, appID).Info("app_scaled", fields...)
	return nil
}

func (s *Service) UpdateAppImage(ctx context.Context, projectID, appID, image string) error {
	if strings.TrimSpace(image) == "" {
		return errors.New("image required")
	}
	p, a, err := s.updateAppDeployment(ctx, projectID, appID, func(dep *appsv1.Deployment) error {
		if len(dep.Spec.Template.Spec.Containers) == 0 {
			dep.Spec.Template.Spec.Containers = []corev1.Container{{Name: "app"}}
		}
		dep.Spec.Template.Spec.Containers[0].Image = image
		return nil
	})
	if err != nil {
		logging.AppErrorLogger(projectID, appID).Error("app_image_update_failed", zap.Error(err), zap.String("image", image))
		return err
	}
	fields := []zap.Field{
		zap.String("app_name", a.Name),
		zap.String("image", image),
		zap.String("cluster_id", p.ClusterID),
		zap.String("namespace", p.Namespace),
	}
	logging.ProjectLogger(projectID).Info("app_image_updated", fields...)
	if _, err := s.AppendProjectEvent(ctx, EventInput{
		ProjectID: projectID,
		AppID:     appID,
		Kind:      "app_image_updated",
		Severity:  SeverityInfo,
		Message:   fmt.Sprintf("updated image for %s", a.Name),
		Meta: map[string]any{
			"image":      image,
			"cluster_id": p.ClusterID,
			"namespace":  p.Namespace,
			"app_name":   a.Name,
		},
	}); err != nil {
		return err
	}
	logging.AppLogger(projectID, appID).Info("app_image_updated", fields...)
	return nil
}

func (s *Service) RolloutRestartApp(ctx context.Context, projectID, appID string) error {
	ts := time.Now().UTC().Format(time.RFC3339)
	p, a, err := s.updateAppDeployment(ctx, projectID, appID, func(dep *appsv1.Deployment) error {
		if dep.Annotations == nil {
			dep.Annotations = map[string]string{}
		}
		dep.Annotations["kubeop.io/redeploy"] = ts
		return nil
	})
	if err != nil {
		logging.AppErrorLogger(projectID, appID).Error("app_rollout_restart_failed", zap.Error(err))
		return err
	}
	fields := []zap.Field{
		zap.String("app_name", a.Name),
		zap.String("cluster_id", p.ClusterID),
		zap.String("namespace", p.Namespace),
		zap.String("redeploy_at", ts),
	}
	logging.ProjectLogger(projectID).Info("app_rollout_restarted", fields...)
	if _, err := s.AppendProjectEvent(ctx, EventInput{
		ProjectID: projectID,
		AppID:     appID,
		Kind:      "app_rollout_restarted",
		Severity:  SeverityInfo,
		Message:   fmt.Sprintf("rollout restarted for %s", a.Name),
		Meta: map[string]any{
			"cluster_id":  p.ClusterID,
			"namespace":   p.Namespace,
			"app_name":    a.Name,
			"redeploy_at": ts,
		},
	}); err != nil {
		return err
	}
	logging.AppLogger(projectID, appID).Info("app_rollout_restarted", fields...)
	return nil
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
	p, a, err := s.updateAppDeployment(ctx, projectID, appID, func(dep *appsv1.Deployment) error {
		if len(dep.Spec.Template.Spec.Containers) == 0 {
			return errors.New("app has no containers to attach configmap")
		}
		AttachConfigMapEnv(&dep.Spec.Template.Spec.Containers[0], name, keys, prefix)
		return nil
	})
	if err != nil {
		logging.AppErrorLogger(projectID, appID).Error("app_configmap_attach_failed", zap.Error(err), zap.String("configmap", name))
		return err
	}
	mode := "envFrom"
	if len(keys) > 0 {
		mode = "keys"
	}
	fields := []zap.Field{
		zap.String("app_name", a.Name),
		zap.String("configmap", name),
		zap.String("mode", mode),
		zap.Any("keys", keys),
		zap.String("prefix", prefix),
		zap.String("cluster_id", p.ClusterID),
		zap.String("namespace", p.Namespace),
	}
	logging.ProjectLogger(projectID).Info("app_configmap_attached", fields...)
	if _, err := s.AppendProjectEvent(ctx, EventInput{
		ProjectID: projectID,
		AppID:     appID,
		Kind:      "app_configmap_attached",
		Severity:  SeverityInfo,
		Message:   fmt.Sprintf("configmap %s attached", name),
		Meta: map[string]any{
			"configmap":  name,
			"mode":       mode,
			"keys":       keys,
			"prefix":     prefix,
			"cluster_id": p.ClusterID,
			"namespace":  p.Namespace,
			"app_name":   a.Name,
		},
	}); err != nil {
		return err
	}
	logging.AppLogger(projectID, appID).Info("app_configmap_attached", fields...)
	return nil
}

// DetachConfigMapFromApp removes ConfigMap references from the app deployment.
func (s *Service) DetachConfigMapFromApp(ctx context.Context, projectID, appID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("name is required")
	}
	p, a, err := s.updateAppDeployment(ctx, projectID, appID, func(dep *appsv1.Deployment) error {
		if len(dep.Spec.Template.Spec.Containers) == 0 {
			return errors.New("app has no containers to detach configmap")
		}
		DetachConfigMapEnv(&dep.Spec.Template.Spec.Containers[0], name)
		return nil
	})
	if err != nil {
		logging.AppErrorLogger(projectID, appID).Error("app_configmap_detach_failed", zap.Error(err), zap.String("configmap", name))
		return err
	}
	fields := []zap.Field{
		zap.String("app_name", a.Name),
		zap.String("configmap", name),
		zap.String("cluster_id", p.ClusterID),
		zap.String("namespace", p.Namespace),
	}
	logging.ProjectLogger(projectID).Info("app_configmap_detached", fields...)
	if _, err := s.AppendProjectEvent(ctx, EventInput{
		ProjectID: projectID,
		AppID:     appID,
		Kind:      "app_configmap_detached",
		Severity:  SeverityInfo,
		Message:   fmt.Sprintf("configmap %s detached", name),
		Meta: map[string]any{
			"configmap":  name,
			"cluster_id": p.ClusterID,
			"namespace":  p.Namespace,
			"app_name":   a.Name,
		},
	}); err != nil {
		return err
	}
	logging.AppLogger(projectID, appID).Info("app_configmap_detached", fields...)
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
	p, a, err := s.updateAppDeployment(ctx, projectID, appID, func(dep *appsv1.Deployment) error {
		if len(dep.Spec.Template.Spec.Containers) == 0 {
			return errors.New("app has no containers to attach secret")
		}
		AttachSecretEnv(&dep.Spec.Template.Spec.Containers[0], name, keys, prefix)
		return nil
	})
	if err != nil {
		logging.AppErrorLogger(projectID, appID).Error("app_secret_attach_failed", zap.Error(err), zap.String("secret", name))
		return err
	}
	mode := "envFrom"
	if len(keys) > 0 {
		mode = "keys"
	}
	fields := []zap.Field{
		zap.String("app_name", a.Name),
		zap.String("secret", name),
		zap.String("mode", mode),
		zap.Any("keys", keys),
		zap.String("prefix", prefix),
		zap.String("cluster_id", p.ClusterID),
		zap.String("namespace", p.Namespace),
	}
	logging.ProjectLogger(projectID).Info("app_secret_attached", fields...)
	if _, err := s.AppendProjectEvent(ctx, EventInput{
		ProjectID: projectID,
		AppID:     appID,
		Kind:      "app_secret_attached",
		Severity:  SeverityInfo,
		Message:   fmt.Sprintf("secret %s attached", name),
		Meta: map[string]any{
			"secret":     name,
			"mode":       mode,
			"keys":       keys,
			"prefix":     prefix,
			"cluster_id": p.ClusterID,
			"namespace":  p.Namespace,
			"app_name":   a.Name,
		},
	}); err != nil {
		return err
	}
	logging.AppLogger(projectID, appID).Info("app_secret_attached", fields...)
	return nil
}

// DetachSecretFromApp removes Secret references from the app deployment.
func (s *Service) DetachSecretFromApp(ctx context.Context, projectID, appID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("name is required")
	}
	p, a, err := s.updateAppDeployment(ctx, projectID, appID, func(dep *appsv1.Deployment) error {
		if len(dep.Spec.Template.Spec.Containers) == 0 {
			return errors.New("app has no containers to detach secret")
		}
		DetachSecretEnv(&dep.Spec.Template.Spec.Containers[0], name)
		return nil
	})
	if err != nil {
		logging.AppErrorLogger(projectID, appID).Error("app_secret_detach_failed", zap.Error(err), zap.String("secret", name))
		return err
	}
	fields := []zap.Field{
		zap.String("app_name", a.Name),
		zap.String("secret", name),
		zap.String("cluster_id", p.ClusterID),
		zap.String("namespace", p.Namespace),
	}
	logging.ProjectLogger(projectID).Info("app_secret_detached", fields...)
	if _, err := s.AppendProjectEvent(ctx, EventInput{
		ProjectID: projectID,
		AppID:     appID,
		Kind:      "app_secret_detached",
		Severity:  SeverityInfo,
		Message:   fmt.Sprintf("secret %s detached", name),
		Meta: map[string]any{
			"secret":     name,
			"cluster_id": p.ClusterID,
			"namespace":  p.Namespace,
			"app_name":   a.Name,
		},
	}); err != nil {
		return err
	}
	logging.AppLogger(projectID, appID).Info("app_secret_detached", fields...)
	return nil
}

type appDeploymentPlan struct {
	AppID         string
	Project       store.Project
	ClusterName   string
	Client        crclient.Client
	KubeName      string
	Replicas      int32
	Resources     map[string]string
	Ports         []AppPort
	Env           map[string]string
	Secrets       []string
	Flavor        string
	Repo          string
	WebhookSecret string
	Domain        string
	Host          string
	SourceType    string
	Image         string
	Manifests     []string
	HelmSpec      map[string]any
	HelmValues    map[string]any
	HelmChart     string
	HelmRendered  string
	RenderedObjs  []RenderedObjectSummary
	LBSummary     LoadBalancerSummary
	Warnings      []string
}

func (s *Service) ValidateApp(ctx context.Context, in AppDeployInput) (AppValidationOutput, error) {
	plan, err := s.planAppDeployment(ctx, in, "")
	if err != nil {
		return AppValidationOutput{}, err
	}
	logging.ProjectLogger(plan.Project.ID).Info(
		"app_validate",
		zap.String("app_name", in.Name),
		zap.String("source", plan.SourceType),
		zap.String("namespace", plan.Project.Namespace),
	)

	resources := cloneStringMap(plan.Resources)
	ports := clonePorts(plan.Ports)
	warnings := append([]string(nil), plan.Warnings...)

	return AppValidationOutput{
		ProjectID:        plan.Project.ID,
		ProjectNamespace: plan.Project.Namespace,
		ClusterID:        plan.Project.ClusterID,
		Source:           plan.SourceType,
		Flavor:           plan.Flavor,
		KubeName:         plan.KubeName,
		Replicas:         plan.Replicas,
		Resources:        resources,
		Ports:            ports,
		Domain:           plan.Host,
		LoadBalancers:    plan.LBSummary,
		RenderedObjects:  append([]RenderedObjectSummary(nil), plan.RenderedObjs...),
		HelmChart:        plan.HelmChart,
		HelmValues:       cloneAnyMap(plan.HelmValues),
		Warnings:         warnings,
	}, nil
}

func (s *Service) planAppDeployment(ctx context.Context, in AppDeployInput, appID string) (*appDeploymentPlan, error) {
	if strings.TrimSpace(in.ProjectID) == "" || strings.TrimSpace(in.Name) == "" {
		return nil, errors.New("projectId and name are required")
	}
	srcCount := 0
	if strings.TrimSpace(in.Image) != "" {
		srcCount++
	}
	if len(in.Manifests) > 0 {
		srcCount++
	}
	if in.Helm != nil {
		srcCount++
	}
	if srcCount != 1 {
		return nil, errors.New("provide exactly one of image, manifests, or helm")
	}

	plan := &appDeploymentPlan{
		AppID:         appID,
		Flavor:        strings.TrimSpace(in.Flavor),
		Repo:          strings.TrimSpace(in.Repo),
		WebhookSecret: strings.TrimSpace(in.WebhookSecret),
		Domain:        strings.TrimSpace(in.Domain),
		Env:           cloneStringMap(in.Env),
		Ports:         clonePorts(in.Ports),
		Secrets:       cloneStringSlice(in.Secrets),
		Image:         strings.TrimSpace(in.Image),
		Manifests:     cloneStringSlice(in.Manifests),
		HelmSpec:      cloneAnyMap(in.Helm),
	}
	plan.KubeName = deriveKubeName(in.Name, appID)
	plan.Project.ID = in.ProjectID

	p, qo, _, err := s.st.GetProject(ctx, in.ProjectID)
	if err != nil {
		return nil, err
	}
	plan.Project = p

	overrides, err := DecodeQuotaOverrides(qo)
	if err != nil {
		return nil, err
	}

	if s.km == nil {
		return nil, errors.New("kube manager not configured")
	}
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
	client, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
	if err != nil {
		return nil, err
	}
	plan.Client = client

	replicas := int32(1)
	if in.Replicas != nil {
		replicas = *in.Replicas
	}
	resources := cloneStringMap(in.Resources)
	if resources == nil {
		resources = map[string]string{}
	}
	if plan.Flavor != "" {
		f, ok := builtinFlavors()[plan.Flavor]
		if !ok {
			return nil, fmt.Errorf("unknown flavor %q", plan.Flavor)
		}
		if in.Replicas == nil {
			replicas = f.Replicas
		}
		if _, ok := resources["requests.cpu"]; !ok {
			resources["requests.cpu"] = f.CPU
		}
		if _, ok := resources["requests.memory"]; !ok {
			resources["requests.memory"] = f.Memory
		}
		if _, ok := resources["limits.cpu"]; !ok {
			resources["limits.cpu"] = f.CPU
		}
		if _, ok := resources["limits.memory"]; !ok {
			resources["limits.memory"] = f.Memory
		}
	}
	plan.Replicas = replicas
	plan.Resources = resources

	lbRequested := 0
	for _, port := range plan.Ports {
		if strings.EqualFold(port.ServiceType, "LoadBalancer") {
			lbRequested++
		}
	}
	existingLB := 0
	if lbRequested > 0 {
		var svcs corev1.ServiceList
		if err := client.List(ctx, &svcs, crclient.InNamespace(p.Namespace)); err != nil {
			plan.Warnings = append(plan.Warnings, fmt.Sprintf("failed to inspect existing services: %v", err))
		} else {
			for _, svc := range svcs.Items {
				if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
					existingLB++
				}
			}
		}
	}
	maxLB := s.cfg.MaxLoadBalancersPerProject
	if v, ok := overrides["services.loadbalancers"]; ok {
		if n, err := parseInt(v); err == nil {
			maxLB = n
		}
	}
	plan.LBSummary = LoadBalancerSummary{Requested: lbRequested, Existing: existingLB, Limit: maxLB}
	if lbRequested > 0 && existingLB+lbRequested > maxLB {
		return nil, fmt.Errorf("exceeds services.loadbalancers quota: %d used, %d requested, max %d", existingLB, lbRequested, maxLB)
	}

	clusterName := p.ClusterID
	if cl, err := s.st.GetCluster(ctx, p.ClusterID); err == nil && strings.TrimSpace(cl.Name) != "" {
		clusterName = cl.Name
	} else if err != nil {
		logging.ProjectLogger(p.ID).Warn("cluster_lookup_failed", zap.String("cluster_id", p.ClusterID), zap.Error(err))
		plan.Warnings = append(plan.Warnings, fmt.Sprintf("cluster lookup failed: %v", err))
	}
	plan.ClusterName = clusterName
	plan.Host = s.computeIngressHost(plan.Domain, p, clusterName, plan.KubeName)

	switch {
	case plan.Image != "":
		plan.SourceType = "image"
		plan.RenderedObjs = append(plan.RenderedObjs, RenderedObjectSummary{Kind: "Deployment", Name: plan.KubeName, Namespace: p.Namespace})
		if len(plan.Ports) > 0 {
			plan.RenderedObjs = append(plan.RenderedObjs, RenderedObjectSummary{Kind: "Service", Name: plan.KubeName, Namespace: p.Namespace})
		}
		if plan.Host != "" && len(plan.Ports) > 0 {
			plan.RenderedObjs = append(plan.RenderedObjs, RenderedObjectSummary{Kind: "Ingress", Name: plan.KubeName, Namespace: p.Namespace})
			if s.cfg.EnableCertManager {
				plan.RenderedObjs = append(plan.RenderedObjs, RenderedObjectSummary{Kind: "Certificate", Name: plan.KubeName + "-cert", Namespace: p.Namespace})
			}
		}
	case len(plan.Manifests) > 0:
		plan.SourceType = "manifests"
		for _, doc := range plan.Manifests {
			objs, err := decodeManifestDocuments(doc)
			if err != nil {
				return nil, fmt.Errorf("validate manifest: %w", err)
			}
			summaries, warns := summariseObjects(objs, p.Namespace)
			plan.RenderedObjs = append(plan.RenderedObjs, summaries...)
			plan.Warnings = append(plan.Warnings, warns...)
		}
	case plan.HelmSpec != nil:
		plan.SourceType = "helm"
		chart := strings.TrimSpace(getString(plan.HelmSpec, "chart"))
		if vals, ok := plan.HelmSpec["values"].(map[string]any); ok {
			plan.HelmValues = cloneAnyMap(vals)
		}
		var rendered string
		if ociRaw, ok := plan.HelmSpec["oci"]; ok {
			if chart != "" {
				return nil, errors.New("helm spec must not set both chart and oci")
			}
			ociMap, ok := ociRaw.(map[string]any)
			if !ok {
				return nil, errors.New("helm.oci must be an object")
			}
			ref := strings.TrimSpace(getString(ociMap, "ref"))
			if ref == "" {
				return nil, errors.New("helm.oci.ref is required")
			}
			plan.HelmChart = ref
			insecure := false
			if v, ok := ociMap["insecure"].(bool); ok {
				insecure = v
			}
			credID := strings.TrimSpace(getString(ociMap, "registryCredentialId"))
			var auth *helmOCIAuth
			if credID != "" {
				host, _, err := parseOCIReference(ref)
				if err != nil {
					return nil, err
				}
				auth, err = s.registryCredentialAuth(ctx, plan.Project, credID, host)
				if err != nil {
					return nil, err
				}
			}
			rendered, err = renderHelmChartFromOCI(ctx, ref, plan.KubeName, p.Namespace, plan.HelmValues, auth, insecure)
			if err != nil {
				return nil, err
			}
		} else {
			if chart == "" {
				return nil, errors.New("helm chart source requires helm.chart or helm.oci")
			}
			plan.HelmChart = chart
			var err error
			rendered, err = renderHelmChartFromURL(ctx, chart, plan.KubeName, p.Namespace, plan.HelmValues)
			if err != nil {
				return nil, err
			}
		}
		plan.HelmRendered = rendered
		objs, err := decodeManifestDocuments(rendered)
		if err != nil {
			return nil, fmt.Errorf("parse rendered helm manifests: %w", err)
		}
		summaries, warns := summariseObjects(objs, p.Namespace)
		plan.RenderedObjs = append(plan.RenderedObjs, summaries...)
		plan.Warnings = append(plan.Warnings, warns...)
	}

	return plan, nil
}

func (s *Service) registryCredentialAuth(ctx context.Context, project store.Project, credentialID, registryHost string) (*helmOCIAuth, error) {
	out, err := s.GetRegistryCredential(ctx, credentialID)
	if err != nil {
		return nil, err
	}
	switch out.ScopeType {
	case CredentialScopeProject:
		if out.ScopeID != project.ID {
			return nil, errors.New("registry credential not accessible to project")
		}
	case CredentialScopeUser:
		if out.ScopeID != project.UserID {
			return nil, errors.New("registry credential not accessible to project user")
		}
	default:
		return nil, errors.New("registry credential scope unsupported")
	}
	host := registryHostFromValue(out.Registry)
	if host != "" && !strings.EqualFold(host, registryHost) {
		return nil, fmt.Errorf("registry credential host %s does not match %s", host, registryHost)
	}
	authType := strings.ToUpper(strings.TrimSpace(out.AuthType))
	auth := &helmOCIAuth{}
	switch authType {
	case "TOKEN":
		token := strings.TrimSpace(out.Secret.Token)
		if token == "" {
			return nil, errors.New("registry credential token is empty")
		}
		auth.Password = token
	case "BASIC":
		username := strings.TrimSpace(out.Username)
		password := strings.TrimSpace(out.Secret.Password)
		if username == "" || password == "" {
			return nil, errors.New("registry credential username and password required for BASIC authType")
		}
		auth.Username = username
		auth.Password = password
	default:
		return nil, fmt.Errorf("registry credential authType %s unsupported for helm oci", out.AuthType)
	}
	if auth.Password == "" {
		return nil, errors.New("registry credential missing secret payload")
	}
	return auth, nil
}

func registryHostFromValue(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(u.Hostname())
}

func (s *Service) updateAppDeployment(ctx context.Context, projectID, appID string, mutate func(*appsv1.Deployment) error) (store.Project, store.App, error) {
	p, _, _, err := s.st.GetProject(ctx, projectID)
	if err != nil {
		return store.Project{}, store.App{}, err
	}
	a, err := s.st.GetApp(ctx, appID)
	if err != nil {
		return store.Project{}, store.App{}, err
	}
	if a.ProjectID != projectID {
		return store.Project{}, store.App{}, errors.New("app does not belong to project")
	}
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
	c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
	if err != nil {
		return store.Project{}, store.App{}, err
	}
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: p.Namespace, Name: appKubeName(a)}}
	if err := c.Get(ctx, crclient.ObjectKey{Namespace: dep.Namespace, Name: dep.Name}, dep); err != nil {
		if !apierrors.IsNotFound(err) {
			return store.Project{}, store.App{}, err
		}
	}
	if err := mutate(dep); err != nil {
		return store.Project{}, store.App{}, err
	}
	if err := apply(ctx, c, dep); err != nil {
		return store.Project{}, store.App{}, err
	}
	return p, a, nil
}
func (s *Service) DeployApp(ctx context.Context, in AppDeployInput) (AppDeployOutput, error) {
	if strings.TrimSpace(in.ProjectID) == "" || strings.TrimSpace(in.Name) == "" {
		return AppDeployOutput{}, errors.New("projectId and name are required")
	}
	appID := uuid.New().String()
	plan, err := s.planAppDeployment(ctx, in, appID)
	if err != nil {
		return AppDeployOutput{}, err
	}
	if fm := logging.Files(); fm != nil {
		if err := fm.EnsureApp(plan.Project.ID, plan.AppID); err != nil {
			logging.AppErrorLogger(plan.Project.ID, plan.AppID).Error("app_log_prepare_failed", zap.Error(err))
			return AppDeployOutput{}, fmt.Errorf("prepare app logs: %w", err)
		}
	}

	p := plan.Project
	c := plan.Client
	kubeName := plan.KubeName

	var svcName, ingName string
	source := plan.SourceType

	switch plan.SourceType {
	case "image":
		dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Namespace: p.Namespace, Name: kubeName, Labels: map[string]string{"kubeop.app-id": plan.AppID, "app.kubernetes.io/name": kubeName}}}
		dep.Spec.Replicas = &plan.Replicas
		dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"kubeop.app-id": plan.AppID}}
		dep.Spec.Template.ObjectMeta.Labels = map[string]string{"kubeop.app-id": plan.AppID, "app.kubernetes.io/name": kubeName}
		ctn := corev1.Container{Name: "app", Image: plan.Image}
		ctn.SecurityContext = DefaultContainerSecurityContext(s.cfg.PodSecurityLevel)
		if len(plan.Resources) > 0 {
			ctn.Resources.Requests = corev1.ResourceList{}
			ctn.Resources.Limits = corev1.ResourceList{}
			if v := plan.Resources["requests.cpu"]; v != "" {
				ctn.Resources.Requests[corev1.ResourceCPU] = resourceMustParse(v)
			}
			if v := plan.Resources["requests.memory"]; v != "" {
				ctn.Resources.Requests[corev1.ResourceMemory] = resourceMustParse(v)
			}
			if v := plan.Resources["limits.cpu"]; v != "" {
				ctn.Resources.Limits[corev1.ResourceCPU] = resourceMustParse(v)
			}
			if v := plan.Resources["limits.memory"]; v != "" {
				ctn.Resources.Limits[corev1.ResourceMemory] = resourceMustParse(v)
			}
		}
		for k, v := range plan.Env {
			ctn.Env = append(ctn.Env, corev1.EnvVar{Name: k, Value: v})
		}
		for _, pr := range plan.Ports {
			if pr.ContainerPort > 0 {
				ctn.Ports = append(ctn.Ports, corev1.ContainerPort{ContainerPort: pr.ContainerPort, Protocol: corev1.ProtocolTCP})
			}
		}
		dep.Spec.Template.Spec.Containers = []corev1.Container{ctn}
		if err := apply(ctx, c, dep); err != nil {
			return AppDeployOutput{}, err
		}
		if len(plan.Secrets) > 0 {
			dep.Spec.Template.Spec.Containers[0].EnvFrom = make([]corev1.EnvFromSource, 0, len(plan.Secrets))
			for _, sref := range plan.Secrets {
				dep.Spec.Template.Spec.Containers[0].EnvFrom = append(dep.Spec.Template.Spec.Containers[0].EnvFrom, corev1.EnvFromSource{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: sref}}})
			}
			if err := apply(ctx, c, dep); err != nil {
				return AppDeployOutput{}, err
			}
		}
		if len(plan.Ports) > 0 {
			svc := &corev1.Service{ObjectMeta: metav1.ObjectMeta{Namespace: p.Namespace, Name: dep.Name, Labels: map[string]string{"kubeop.app-id": plan.AppID}}}
			svc.Annotations = s.lbServiceAnnotations()
			for _, pr := range plan.Ports {
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
			svc.Spec.Selector = map[string]string{"kubeop.app-id": plan.AppID}
			if err := apply(ctx, c, svc); err != nil {
				return AppDeployOutput{}, err
			}
			svcName = svc.Name
		}
		host := plan.Host
		if host != "" && len(plan.Ports) > 0 {
			httpPort := int32(80)
			for _, pr := range plan.Ports {
				if pr.ServicePort == 80 || pr.ServicePort == 8080 {
					httpPort = pr.ServicePort
					break
				}
			}
			ing := &netv1.Ingress{ObjectMeta: metav1.ObjectMeta{Namespace: p.Namespace, Name: kubeName, Labels: map[string]string{"kubeop.app-id": plan.AppID}}}
			pathType := netv1.PathTypePrefix
			ing.Spec.Rules = []netv1.IngressRule{{
				Host: host,
				IngressRuleValue: netv1.IngressRuleValue{HTTP: &netv1.HTTPIngressRuleValue{Paths: []netv1.HTTPIngressPath{{
					Path:     "/",
					PathType: &pathType,
					Backend:  netv1.IngressBackend{Service: &netv1.IngressServiceBackend{Name: kubeName, Port: netv1.ServiceBackendPort{Number: httpPort}}},
				}}}},
			}}
			if s.cfg.EnableCertManager {
				secretName := kubeName + "-tls"
				ing.Spec.TLS = []netv1.IngressTLS{{Hosts: []string{host}, SecretName: secretName}}
				if ing.Annotations == nil {
					ing.Annotations = map[string]string{}
				}
				ing.Annotations["cert-manager.io/cluster-issuer"] = "letsencrypt-prod"
				ing.Annotations["kubernetes.io/tls-acme"] = "true"
				cert := &unstructured.Unstructured{}
				cert.SetGroupVersionKind(schema.GroupVersionKind{Group: "cert-manager.io", Version: "v1", Kind: "Certificate"})
				cert.SetNamespace(p.Namespace)
				cert.SetName(kubeName + "-cert")
				cert.Object["spec"] = map[string]any{
					"dnsNames":   []string{host},
					"secretName": secretName,
					"issuerRef":  map[string]any{"name": "letsencrypt-prod", "kind": "ClusterIssuer"},
				}
				_ = apply(ctx, c, cert)
			}
			if err := apply(ctx, c, ing); err != nil {
				return AppDeployOutput{}, err
			}
			ingName = ing.Name
			if host != "" {
				if _, err := s.st.UpsertAppDomain(ctx, plan.AppID, host, "pending"); err != nil {
					logging.ProjectLogger(plan.Project.ID).Warn("app_domain_store_failed", zap.String("fqdn", host), zap.Error(err))
				}
			}
			_ = s.ensureDNSForService(ctx, p.ID, plan.AppID, p.ClusterID, p.Namespace, svcName, host)
		}

	case "manifests":
		for _, doc := range plan.Manifests {
			if err := s.applyRawManifest(ctx, p.ClusterID, []byte(doc), p.Namespace, map[string]string{"kubeop.app-id": plan.AppID}); err != nil {
				return AppDeployOutput{}, err
			}
		}
	case "helm":
		if strings.TrimSpace(plan.HelmRendered) == "" {
			return AppDeployOutput{}, errors.New("helm render unexpectedly empty")
		}
		if err := s.applyRawManifest(ctx, p.ClusterID, []byte(plan.HelmRendered), p.Namespace, map[string]string{"kubeop.app-id": plan.AppID}); err != nil {
			return AppDeployOutput{}, err
		}
	default:
		return AppDeployOutput{}, fmt.Errorf("unsupported source type %q", plan.SourceType)
	}

	if err := s.st.CreateApp(ctx, plan.AppID, plan.Project.ID, in.Name, "deployed", plan.Repo, plan.WebhookSecret, "", map[string]any{"image": plan.Image, "ports": plan.Ports, "env": plan.Env, "helm": plan.HelmSpec, "kubeName": kubeName}); err != nil {
		logging.AppErrorLogger(plan.Project.ID, plan.AppID).Error("app_persist_failed", zap.Error(err))
		logging.L().Warn("store app create failed", zap.String("error", err.Error()))
		return AppDeployOutput{}, fmt.Errorf("persist app: %w", err)
	}

	releaseID, err := s.recordRelease(ctx, plan, in)
	if err != nil {
		return AppDeployOutput{}, fmt.Errorf("record release: %w", err)
	}

	fields := []zap.Field{
		zap.String("app_id", plan.AppID),
		zap.String("app_name", in.Name),
		zap.String("kube_name", kubeName),
		zap.String("cluster_id", p.ClusterID),
		zap.String("namespace", p.Namespace),
		zap.String("source", source),
		zap.String("release_id", releaseID),
	}
	if svcName != "" {
		fields = append(fields, zap.String("service_name", svcName))
	}
	if ingName != "" {
		fields = append(fields, zap.String("ingress_name", ingName))
	}
	logging.ProjectLogger(plan.Project.ID).Info("app_deployed", fields...)

	meta := map[string]any{
		"app_id":     plan.AppID,
		"app_name":   in.Name,
		"kube_name":  kubeName,
		"cluster_id": p.ClusterID,
		"namespace":  p.Namespace,
		"source":     source,
		"release_id": releaseID,
	}
	if svcName != "" {
		meta["service_name"] = svcName
	}
	if ingName != "" {
		meta["ingress_name"] = ingName
	}
	if _, err := s.AppendProjectEvent(ctx, EventInput{
		ProjectID: plan.Project.ID,
		AppID:     plan.AppID,
		Kind:      "app_deployed",
		Severity:  SeverityInfo,
		Message:   fmt.Sprintf("app %s deployed", in.Name),
		Meta:      meta,
	}); err != nil {
		return AppDeployOutput{}, err
	}
	logging.AppLogger(plan.Project.ID, plan.AppID).Info("app_deployed", fields...)
	return AppDeployOutput{AppID: plan.AppID, Name: in.Name, Service: svcName, Ingress: ingName}, nil
}

// computeIngressHost returns domain as-is if provided, else generates from env if enabled.
func (s *Service) computeIngressHost(domain string, project store.Project, clusterName, app string) string {
	if domain != "" {
		return domain
	}
	if !s.cfg.PaaSWildcardEnabled || s.cfg.PaaSDomain == "" || app == "" {
		return ""
	}
	projectSegment := util.Slugify(project.Name)
	if projectSegment == "" {
		projectSegment = util.Slugify(project.Namespace)
	}
	clusterSegment := util.Slugify(clusterName)
	if clusterSegment == "" {
		clusterSegment = util.Slugify(project.ClusterID)
	}
	if projectSegment == "" || clusterSegment == "" {
		return ""
	}
	return fmt.Sprintf("%s.%s.%s.%s", app, projectSegment, clusterSegment, s.cfg.PaaSDomain)
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
	objs, err := decodeManifestDocuments(string(raw))
	if err != nil {
		return err
	}
	if len(objs) == 0 {
		return nil
	}
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, clusterID) }
	c, err := s.km.GetOrCreate(ctx, clusterID, loader)
	if err != nil {
		return err
	}
	for i := range objs {
		obj := objs[i]
		if isNamespacedKind(obj.GetKind()) && obj.GetNamespace() == "" {
			obj.SetNamespace(namespace)
		}
		meta := obj.GetLabels()
		if meta == nil {
			meta = map[string]string{}
		}
		for k, v := range labels {
			meta[k] = v
		}
		obj.SetLabels(meta)
		if err := apply(ctx, c, &obj); err != nil {
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
	secret, err := s.mintServiceAccountSecret(ctx, cs, p.Namespace, saName)
	if err != nil {
		return KubeconfigRenewOutput{}, err
	}
	// Rebuild kubeconfig preserving cluster values
	clusterKc, err := s.DecryptClusterKubeconfig(ctx, p.ClusterID)
	if err != nil {
		return KubeconfigRenewOutput{}, err
	}
	server := extractServer(clusterKc)
	caB64 := base64.StdEncoding.EncodeToString(secret.Data["ca.crt"])
	token := string(secret.Data[corev1.ServiceAccountTokenKey])
	kc, err := buildNamespaceScopedKubeconfig(server, caB64, p.Namespace, saName, p.ClusterID, token)
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
	if existing, _, err := s.st.GetKubeconfigByProject(ctx, projectID); err == nil {
		if err := s.st.UpdateKubeconfigRecord(ctx, existing.ID, secret.Name, saName, enc); err != nil {
			return KubeconfigRenewOutput{}, err
		}
	} else if errors.Is(err, sql.ErrNoRows) {
		pid := projectID
		rec := store.KubeconfigRecord{
			ID:             uuid.New().String(),
			ClusterID:      p.ClusterID,
			Namespace:      p.Namespace,
			UserID:         p.UserID,
			ProjectID:      &pid,
			ServiceAccount: saName,
			SecretName:     secret.Name,
		}
		if _, err := s.st.CreateKubeconfigRecord(ctx, rec, enc); err != nil {
			return KubeconfigRenewOutput{}, err
		}
	}
	fields := []zap.Field{
		zap.String("cluster_id", p.ClusterID),
		zap.String("namespace", p.Namespace),
	}
	logging.ProjectLogger(projectID).Info("project_kubeconfig_renewed", fields...)
	if _, err := s.AppendProjectEvent(ctx, EventInput{
		ProjectID: projectID,
		Kind:      "project_kubeconfig_renewed",
		Severity:  SeverityInfo,
		Message:   "project kubeconfig renewed",
		Meta: map[string]any{
			"cluster_id": p.ClusterID,
			"namespace":  p.Namespace,
		},
	}); err != nil {
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
				logging.L().Warn("webhook signature invalid", zap.String("repo", repo), zap.String("app", ap.Name))
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

// DefaultContainerSecurityContext returns opinionated security defaults that align with the
// configured Pod Security Admission level. When running in "restricted" mode the container is
// forced to run as non-root with a read-only root filesystem and dropped capabilities. For
// more permissive levels (baseline, privileged, or empty) the helper still disables privilege
// escalation and keeps a runtime/default seccomp profile, but it allows the image to manage the
// user, filesystem, and capabilities so common upstream images continue to run.
func DefaultContainerSecurityContext(level string) *corev1.SecurityContext {
	lvl := strings.ToLower(strings.TrimSpace(level))
	noPrivEsc := false
	prof := corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}
	sc := &corev1.SecurityContext{
		AllowPrivilegeEscalation: &noPrivEsc,
		SeccompProfile:           &prof,
	}
	if lvl == "restricted" {
		nonRoot := true
		roRoot := true
		sc.RunAsNonRoot = &nonRoot
		sc.ReadOnlyRootFilesystem = &roRoot
		sc.Capabilities = &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}
	}
	return sc
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

func decodeManifestDocuments(raw string) ([]unstructured.Unstructured, error) {
	parts := splitYAMLDocs(raw)
	objs := make([]unstructured.Unstructured, 0, len(parts))
	for _, doc := range parts {
		if strings.TrimSpace(doc) == "" {
			continue
		}
		js, err := yaml.YAMLToJSON([]byte(doc))
		if err != nil {
			return nil, err
		}
		var u unstructured.Unstructured
		if err := u.UnmarshalJSON(js); err != nil {
			return nil, err
		}
		objs = append(objs, u)
	}
	return objs, nil
}

func summariseObjects(objs []unstructured.Unstructured, defaultNamespace string) ([]RenderedObjectSummary, []string) {
	if len(objs) == 0 {
		return nil, nil
	}
	summaries := make([]RenderedObjectSummary, 0, len(objs))
	var warnings []string
	for _, obj := range objs {
		kind := obj.GetKind()
		name := strings.TrimSpace(obj.GetName())
		if name == "" {
			name = strings.TrimSpace(obj.GetGenerateName())
		}
		if name == "" {
			warnings = append(warnings, fmt.Sprintf("object kind %s is missing name", kind))
		}
		ns := obj.GetNamespace()
		if ns == "" && isNamespacedKind(kind) {
			ns = defaultNamespace
		}
		summaries = append(summaries, RenderedObjectSummary{Kind: kind, Name: name, Namespace: ns})
	}
	return summaries, warnings
}

func cloneStringSlice(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func clonePorts(in []AppPort) []AppPort {
	if len(in) == 0 {
		return nil
	}
	out := make([]AppPort, len(in))
	copy(out, in)
	return out
}

func cloneAnyMap(in map[string]any) map[string]any {
	if in == nil {
		return nil
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = cloneAnyValue(v)
	}
	return out
}

func cloneAnySlice(in []any) []any {
	if len(in) == 0 {
		return nil
	}
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = cloneAnyValue(v)
	}
	return out
}

func cloneAnyValue(v any) any {
	switch tv := v.(type) {
	case map[string]any:
		return cloneAnyMap(tv)
	case []any:
		return cloneAnySlice(tv)
	case []string:
		return cloneStringSlice(tv)
	case map[string]string:
		return cloneStringMap(tv)
	default:
		return v
	}
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

type HelmRegistryClient interface {
	Pull(ref string, options ...registry.PullOption) (*registry.PullResult, error)
	Login(host string, options ...registry.LoginOption) error
}

type helmRegistryClientFactoryFunc func(host string, addrs []netip.Addr, insecure bool) (HelmRegistryClient, error)

type helmOCIAuth struct {
	Username string
	Password string
}

var (
	helmHostResolverMu sync.RWMutex
	helmHostResolver   = defaultHelmHostResolver

	helmHTTPClientMu sync.RWMutex
	helmHTTPClient   = newDefaultHelmChartHTTPClient()

	helmDialFuncMu sync.RWMutex
	helmDialFunc   = defaultHelmDialFunc()

	helmRegistryFactoryMu sync.RWMutex
	helmRegistryFactory   helmRegistryClientFactoryFunc = defaultHelmRegistryClientFactory
)

func defaultHelmRegistryClientFactory(host string, addrs []netip.Addr, insecure bool) (HelmRegistryClient, error) {
	opts := []registry.ClientOption{
		registry.ClientOptHTTPClient(newHelmRegistryHTTPClient(host, addrs)),
	}
	if insecure {
		opts = append(opts, registry.ClientOptPlainHTTP())
	}
	return registry.NewClient(opts...)
}

func newHelmRegistryClient(host string, addrs []netip.Addr, insecure bool) (HelmRegistryClient, error) {
	helmRegistryFactoryMu.RLock()
	factory := helmRegistryFactory
	helmRegistryFactoryMu.RUnlock()
	return factory(host, addrs, insecure)
}

// SetHelmRegistryClientFactory swaps the factory used to create Helm OCI registry clients.
// It returns a restore function to reset the previous factory.
func SetHelmRegistryClientFactory(factory helmRegistryClientFactoryFunc) func() {
	if factory == nil {
		factory = defaultHelmRegistryClientFactory
	}
	helmRegistryFactoryMu.Lock()
	prev := helmRegistryFactory
	helmRegistryFactory = factory
	helmRegistryFactoryMu.Unlock()

	return func() {
		helmRegistryFactoryMu.Lock()
		helmRegistryFactory = prev
		helmRegistryFactoryMu.Unlock()
	}
}

func defaultHelmHostResolver(ctx context.Context, host string) ([]net.IP, error) {
	return net.DefaultResolver.LookupIP(ctx, "ip", host)
}

func defaultHelmDialFunc() func(context.Context, string, string) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	return dialer.DialContext
}

func newHelmRegistryHTTPClient(host string, addrs []netip.Addr) *http.Client {
	allowed := make(map[string]struct{}, len(addrs))
	for _, addr := range addrs {
		allowed[addr.String()] = struct{}{}
	}
	dialer := &net.Dialer{Timeout: 15 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				hostPart, port, err := net.SplitHostPort(address)
				if err != nil {
					return nil, err
				}
				ip := net.ParseIP(hostPart)
				if ip == nil {
					return nil, fmt.Errorf("helm oci registry dial: non-ip address %s", hostPart)
				}
				addr, ok := netip.AddrFromSlice(ip)
				if !ok {
					return nil, fmt.Errorf("helm oci registry dial: invalid ip %s", hostPart)
				}
				if err := ensureHelmChartAddrAllowed(host, addr); err != nil {
					return nil, err
				}
				if _, ok := allowed[addr.String()]; !ok {
					return nil, fmt.Errorf("helm oci registry dial: %s not in allowed targets", addr.String())
				}
				return dialer.DialContext(ctx, network, net.JoinHostPort(addr.String(), port))
			},
			ForceAttemptHTTP2: true,
		},
	}
}

type helmDialContextKey struct{}

type helmDialContext struct {
	host  string
	addrs []netip.Addr
}

func withHelmDialAddrs(ctx context.Context, host string, addrs []netip.Addr) context.Context {
	return context.WithValue(ctx, helmDialContextKey{}, helmDialContext{host: strings.ToLower(host), addrs: addrs})
}

func helmDialAddrsFromContext(ctx context.Context, host string) []netip.Addr {
	v, _ := ctx.Value(helmDialContextKey{}).(helmDialContext)
	if v.host == "" {
		return nil
	}
	if !strings.EqualFold(v.host, host) {
		return nil
	}
	return v.addrs
}

func ensureHelmChartAddrAllowed(host string, addr netip.Addr) error {
	if !addr.IsValid() {
		return fmt.Errorf("helm chart url host %s resolved to invalid ip", host)
	}
	if !addr.IsGlobalUnicast() || addr.IsPrivate() || addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() || addr.IsMulticast() {
		return fmt.Errorf("helm chart url host %s resolved to disallowed network %s", host, addr.String())
	}
	return nil
}

func resolveHelmChartTarget(ctx context.Context, host string) ([]netip.Addr, error) {
	if host == "" {
		return nil, errors.New("helm chart url missing hostname")
	}
	if ip := net.ParseIP(host); ip != nil {
		addr, ok := netip.AddrFromSlice(ip)
		if !ok {
			return nil, fmt.Errorf("helm chart url host %s resolved to invalid ip", host)
		}
		addr = addr.Unmap()
		if err := ensureHelmChartAddrAllowed(host, addr); err != nil {
			return nil, err
		}
		return []netip.Addr{addr}, nil
	}
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
	addrs := make([]netip.Addr, 0, len(ips))
	for _, ipAddr := range ips {
		addr, ok := netip.AddrFromSlice(ipAddr)
		if !ok {
			return nil, fmt.Errorf("resolve helm chart host %s: invalid ip result", host)
		}
		addr = addr.Unmap()
		if err := ensureHelmChartAddrAllowed(host, addr); err != nil {
			return nil, err
		}
		addrs = append(addrs, addr)
	}
	return addrs, nil
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
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
				host, port, err := net.SplitHostPort(address)
				if err != nil {
					return nil, err
				}
				allowed := helmDialAddrsFromContext(ctx, host)
				if len(allowed) == 0 {
					allowed, err = resolveHelmChartTarget(ctx, host)
					if err != nil {
						return nil, err
					}
				}
				dial, err := helmDialFuncForRequest()
				if err != nil {
					return nil, err
				}
				var lastErr error
				for _, addr := range allowed {
					dialAddr := net.JoinHostPort(addr.String(), port)
					conn, err := dial(ctx, network, dialAddr)
					if err == nil {
						return conn, nil
					}
					if ctx.Err() != nil {
						return nil, err
					}
					lastErr = err
				}
				if lastErr != nil {
					return nil, lastErr
				}
				return nil, fmt.Errorf("helm chart dial: no allowed address succeeded for host %s", host)
			},
			ForceAttemptHTTP2: true,
		},
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

func helmDialFuncForRequest() (func(context.Context, string, string) (net.Conn, error), error) {
	helmDialFuncMu.RLock()
	f := helmDialFunc
	helmDialFuncMu.RUnlock()
	if f == nil {
		return nil, errors.New("helm chart dial function not configured")
	}
	return f, nil
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

// SetHelmChartDialFunc swaps the dial function used by the Helm HTTP client transport.
// It returns a restore function to reset the previous dialer.
func SetHelmChartDialFunc(dial func(context.Context, string, string) (net.Conn, error)) func() {
	if dial == nil {
		dial = defaultHelmDialFunc()
	}
	helmDialFuncMu.Lock()
	prev := helmDialFunc
	helmDialFunc = dial
	helmDialFuncMu.Unlock()
	return func() {
		helmDialFuncMu.Lock()
		helmDialFunc = prev
		helmDialFuncMu.Unlock()
	}
}

func getHelmChartHTTPClient() *http.Client {
	helmHTTPClientMu.RLock()
	client := helmHTTPClient
	helmHTTPClientMu.RUnlock()
	return client
}

// ParseHelmChartURL validates the raw input and returns the parsed URL and allowed target addresses.
func ParseHelmChartURL(ctx context.Context, raw string) (*url.URL, []netip.Addr, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, nil, fmt.Errorf("invalid helm chart url: %w", err)
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return nil, nil, fmt.Errorf("helm chart url must use http or https")
	}
	if parsed.Opaque != "" {
		return nil, nil, fmt.Errorf("helm chart url must be absolute")
	}
	if parsed.Host == "" {
		return nil, nil, errors.New("helm chart url must include a host")
	}
	if parsed.User != nil {
		return nil, nil, errors.New("helm chart url must not contain credentials")
	}
	if port := parsed.Port(); port != "" {
		switch parsed.Scheme {
		case "https":
			if port != "443" {
				return nil, nil, fmt.Errorf("helm chart https url must use port 443")
			}
		case "http":
			if port != "80" {
				return nil, nil, fmt.Errorf("helm chart http url must use port 80")
			}
		}
	}
	parsed.Fragment = ""

	host := parsed.Hostname()
	addrs, err := resolveHelmChartTarget(ctx, host)
	if err != nil {
		return nil, nil, err
	}
	return sanitizeHelmChartURL(parsed), addrs, nil
}

// ValidateHelmChartURL ensures Helm chart downloads only use permitted network targets.
func ValidateHelmChartURL(ctx context.Context, raw string) (*url.URL, error) {
	parsed, _, err := ParseHelmChartURL(ctx, raw)
	return parsed, err
}

func sanitizeHelmChartURL(parsed *url.URL) *url.URL {
	sanitized := &url.URL{
		Scheme:   parsed.Scheme,
		Host:     parsed.Host,
		Path:     parsed.EscapedPath(),
		RawPath:  parsed.RawPath,
		RawQuery: parsed.Query().Encode(),
	}
	if parsed.RawQuery == "" {
		sanitized.RawQuery = ""
	}
	return sanitized
}

// renderHelmChartFromURL downloads a chart .tgz and renders manifests using provided values.
func renderHelmChartTemplates(ch *chart.Chart, releaseName, namespace string, values map[string]any) (string, error) {
	if values == nil {
		values = map[string]any{}
	}
	vals, err := chartutil.ToRenderValues(ch, values, chartutil.ReleaseOptions{
		Name: releaseName, Namespace: namespace, IsInstall: true, IsUpgrade: false,
	}, chartutil.DefaultCapabilities)
	if err != nil {
		return "", fmt.Errorf("render helm chart values: %w", err)
	}
	rendered, err := engine.Render(ch, vals)
	if err != nil {
		return "", fmt.Errorf("render helm chart templates: %w", err)
	}
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

func parseOCIReference(ref string) (string, string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", "", errors.New("helm oci ref is required")
	}
	if !strings.HasPrefix(ref, "oci://") {
		return "", "", errors.New("helm oci ref must start with oci://")
	}
	trimmed := strings.TrimPrefix(ref, "oci://")
	parts := strings.SplitN(trimmed, "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[1]) == "" {
		return "", "", errors.New("helm oci ref must include a repository path")
	}
	host := strings.TrimSpace(parts[0])
	if host == "" {
		return "", "", errors.New("helm oci ref missing registry host")
	}
	repo := strings.TrimSpace(parts[1])
	return host, repo, nil
}

func renderHelmChartFromOCI(ctx context.Context, ref, releaseName, namespace string, values map[string]any, auth *helmOCIAuth, insecure bool) (string, error) {
	host, repo, err := parseOCIReference(ref)
	if err != nil {
		return "", err
	}
	if !strings.Contains(repo, ":") && !strings.Contains(repo, "@") {
		return "", errors.New("helm oci ref must include a tag or digest")
	}
	addrs, err := resolveHelmChartTarget(ctx, host)
	if err != nil {
		return "", err
	}
	client, err := newHelmRegistryClient(host, addrs, insecure)
	if err != nil {
		return "", fmt.Errorf("helm oci client: %w", err)
	}
	if auth != nil && (auth.Username != "" || auth.Password != "") {
		loginOpts := []registry.LoginOption{registry.LoginOptBasicAuth(auth.Username, auth.Password)}
		if insecure {
			loginOpts = append(loginOpts, registry.LoginOptInsecure(true))
		}
		if err := client.Login(host, loginOpts...); err != nil {
			return "", fmt.Errorf("helm oci login: %w", err)
		}
	}
	logging.L().With(
		zap.String("registry", host),
		zap.Bool("oci_insecure", insecure),
	).Info("downloading helm chart (oci)")
	result, err := client.Pull(ref)
	if err != nil {
		return "", fmt.Errorf("helm oci pull: %w", err)
	}
	if result == nil || result.Chart == nil || len(result.Chart.Data) == 0 {
		return "", errors.New("helm oci pull returned empty chart data")
	}
	ch, err := loader.LoadArchive(bytes.NewReader(result.Chart.Data))
	if err != nil {
		return "", fmt.Errorf("load helm chart archive: %w", err)
	}
	return renderHelmChartTemplates(ch, releaseName, namespace, values)
}

func renderHelmChartFromURL(ctx context.Context, chartURL, releaseName, namespace string, values map[string]any) (string, error) {
	parsedURL, allowedAddrs, err := ParseHelmChartURL(ctx, chartURL)
	if err != nil {
		return "", fmt.Errorf("validate helm chart url: %w", err)
	}

	reqCtx := withHelmDialAddrs(ctx, parsedURL.Hostname(), allowedAddrs)
	logging.L().With(
		zap.String("scheme", parsedURL.Scheme),
		zap.String("host", parsedURL.Hostname()),
	).Info("downloading helm chart")

	safeURL := parsedURL.String()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, safeURL, nil)
	if err != nil {
		return "", err
	}

	client := getHelmChartHTTPClient()

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download helm chart request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("download chart failed: %s", resp.Status)
	}
	by, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read helm chart body: %w", err)
	}
	ch, err := loader.LoadArchive(bytes.NewReader(by))
	if err != nil {
		return "", fmt.Errorf("load helm chart archive: %w", err)
	}
	return renderHelmChartTemplates(ch, releaseName, namespace, values)
}

// RenderHelmChartFromURLForTest exposes the Helm renderer for integration tests.
func RenderHelmChartFromURLForTest(ctx context.Context, chartURL, releaseName, namespace string, values map[string]any) (string, error) {
	return renderHelmChartFromURL(ctx, chartURL, releaseName, namespace, values)
}

// RenderHelmChartFromOCIForTest exposes the OCI Helm renderer for integration tests.
func RenderHelmChartFromOCIForTest(ctx context.Context, ref, releaseName, namespace string, values map[string]any, username, password string, insecure bool) (string, error) {
	var auth *helmOCIAuth
	if strings.TrimSpace(username) != "" || strings.TrimSpace(password) != "" {
		auth = &helmOCIAuth{Username: username, Password: password}
	}
	return renderHelmChartFromOCI(ctx, ref, releaseName, namespace, values, auth, insecure)
}

// ensureDNSForService finds the LB IP for a Service and calls DNS provider to upsert host -> IP.
func (s *Service) ensureDNSForService(ctx context.Context, projectID, appID, clusterID, namespace, serviceName, host string) error {
	if host == "" {
		return nil
	}
	if s.km == nil {
		return errors.New("kube manager unavailable for DNS automation")
	}
	prov := s.dnsProviderFactory(s.cfg)
	if prov == nil {
		return nil
	}
	logger := s.dnsLogger(projectID, appID, clusterID, namespace, serviceName, host)
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, clusterID) }
	cs, err := s.km.GetClientset(ctx, clusterID, loader)
	if err != nil {
		return DNSError("load kube client", clusterID, namespace, serviceName, host, err)
	}
	svc, err := cs.CoreV1().Services(namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if err != nil {
		return DNSError("fetch service", clusterID, namespace, serviceName, host, err)
	}
	if svc.Spec.Type != corev1.ServiceTypeLoadBalancer {
		return nil
	}
	if addrs := collectLoadBalancerAddrs(svc); len(addrs) > 0 {
		if err := prov.EnsureRecords(host, addrs, s.cfg.DNSRecordTTL); err != nil {
			wrapped := DNSError("ensure dns record", clusterID, namespace, serviceName, host, err)
			logger.Error("dns_record_upsert_failed",
				zap.Error(wrapped),
				zap.Strings("ips", addrsToStrings(addrs)),
				zap.Int("ttl", s.cfg.DNSRecordTTL),
				zap.String("mode", "sync"),
			)
			return wrapped
		}
		logger.Info("dns_record_upserted",
			zap.Strings("ips", addrsToStrings(addrs)),
			zap.Int("ttl", s.cfg.DNSRecordTTL),
			zap.String("mode", "sync"),
		)
		return nil
	}
	logger.Info("dns_wait_for_load_balancer_ip",
		zap.Duration("poll_interval", loadBalancerPollInterval),
		zap.Duration("timeout", loadBalancerPollTimeout),
	)
	go s.waitForServiceDNS(context.Background(), logger, projectID, appID, clusterID, namespace, serviceName, host)
	return nil
}

const (
	loadBalancerPollInterval = 5 * time.Second
	loadBalancerPollTimeout  = 2 * time.Minute
)

type serviceGetter interface {
	Get(ctx context.Context, name string, options metav1.GetOptions) (*corev1.Service, error)
}

func (s *Service) waitForServiceDNS(parentCtx context.Context, logger *zap.Logger, projectID, appID, clusterID, namespace, serviceName, host string) {
	ctx, cancel := context.WithTimeout(parentCtx, loadBalancerPollTimeout)
	defer cancel()
	if s.km == nil {
		return
	}
	prov := s.dnsProviderFactory(s.cfg)
	if prov == nil {
		return
	}
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, clusterID) }
	start := time.Now()
	cs, err := s.km.GetClientset(ctx, clusterID, loader)
	if err != nil {
		wrapped := DNSError("load kube client", clusterID, namespace, serviceName, host, err)
		logger.Error("dns_wait_client_error", zap.Error(wrapped))
		return
	}
	addrs, err := waitForLoadBalancerAddrs(ctx, cs.CoreV1().Services(namespace), serviceName, loadBalancerPollInterval)
	if err != nil {
		wrapped := DNSError("wait for load balancer ip", clusterID, namespace, serviceName, host, err)
		logger.Error("dns_wait_failed",
			zap.Error(wrapped),
			zap.Duration("elapsed", time.Since(start)),
		)
		return
	}
	if err := prov.EnsureRecords(host, addrs, s.cfg.DNSRecordTTL); err != nil {
		wrapped := DNSError("ensure dns record", clusterID, namespace, serviceName, host, err)
		logger.Error("dns_record_upsert_failed",
			zap.Error(wrapped),
			zap.Strings("ips", addrsToStrings(addrs)),
			zap.Int("ttl", s.cfg.DNSRecordTTL),
			zap.Duration("elapsed", time.Since(start)),
			zap.String("mode", "async"),
		)
		return
	}
	logger.Info("dns_record_upserted",
		zap.Strings("ips", addrsToStrings(addrs)),
		zap.Int("ttl", s.cfg.DNSRecordTTL),
		zap.Duration("elapsed", time.Since(start)),
		zap.String("mode", "async"),
	)
}

func waitForLoadBalancerAddrs(ctx context.Context, svcClient serviceGetter, name string, pollInterval time.Duration) ([]netip.Addr, error) {
	if pollInterval <= 0 {
		pollInterval = loadBalancerPollInterval
	}
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()
	for {
		svc, err := svcClient.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if addrs := collectLoadBalancerAddrs(svc); len(addrs) > 0 {
			return addrs, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

// WaitForLoadBalancerIPForTest exposes waitForLoadBalancerAddrs for integration tests.
func WaitForLoadBalancerIPForTest(ctx context.Context, svcClient serviceGetter, name string, pollInterval time.Duration) ([]netip.Addr, error) {
	return waitForLoadBalancerAddrs(ctx, svcClient, name, pollInterval)
}

func collectLoadBalancerAddrs(svc *corev1.Service) []netip.Addr {
	if svc == nil {
		return nil
	}
	var out []netip.Addr
	for _, ing := range svc.Status.LoadBalancer.Ingress {
		if ip := strings.TrimSpace(ing.IP); ip != "" {
			if addr, err := netip.ParseAddr(ip); err == nil {
				out = append(out, addr)
			}
		}
	}
	return out
}

func addrsToStrings(addrs []netip.Addr) []string {
	out := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		out = append(out, addr.String())
	}
	return out
}

// DeleteApp deletes app resources in Kubernetes (by label) and soft-deletes the app row in DB.
func (s *Service) DeleteApp(ctx context.Context, projectID, appID string) error {
	a, err := s.st.GetApp(ctx, appID)
	if err != nil {
		logging.AppErrorLogger(projectID, appID).Error("app_delete_failed", zap.Error(err))
		return err
	}
	if a.ProjectID != projectID {
		return errors.New("app does not belong to project")
	}
	// Load project to know namespace and cluster
	p, _, _, err := s.st.GetProject(ctx, projectID)
	if err != nil {
		logging.AppErrorLogger(projectID, appID).Error("app_delete_failed", zap.Error(err))
		return err
	}
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, p.ClusterID) }
	c, err := s.km.GetOrCreate(ctx, p.ClusterID, loader)
	if err != nil {
		logging.AppErrorLogger(projectID, appID).Error("app_delete_failed", zap.Error(err))
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
	// TLS secret + certificate (if managed)
	appSlug := appKubeName(a)
	if s.cfg.EnableCertManager && appSlug != "" {
		tlsSecret := &corev1.Secret{}
		tlsSecret.Namespace = p.Namespace
		tlsSecret.Name = appSlug + "-tls"
		_ = c.Delete(ctx, tlsSecret)

		cert := &unstructured.Unstructured{}
		cert.SetGroupVersionKind(schema.GroupVersionKind{Group: "cert-manager.io", Version: "v1", Kind: "Certificate"})
		cert.SetNamespace(p.Namespace)
		cert.SetName(appSlug + "-cert")
		_ = c.Delete(ctx, cert)
	}
	// DNS cleanup
	if len(ingHosts) > 0 {
		prov := dns.NewProvider(s.cfg)
		if prov != nil {
			for _, h := range ingHosts {
				_ = prov.DeleteRecords(h)
			}
		}
	}
	if err := s.st.DeleteAppDomains(ctx, appID); err != nil {
		logging.AppErrorLogger(projectID, appID).Warn("app_domain_cleanup_failed", zap.Error(err))
	}
	// Soft-delete in DB (ignore missing)
	_ = s.st.SoftDeleteApp(ctx, appID)
	fields := []zap.Field{
		zap.String("cluster_id", p.ClusterID),
		zap.String("namespace", p.Namespace),
		zap.Int("ingress_hosts_removed", len(ingHosts)),
		zap.Int("domains_removed", len(ingHosts)),
		zap.String("app_name", a.Name),
	}
	logging.ProjectLogger(projectID).Info("app_deleted", fields...)
	if _, err := s.AppendProjectEvent(ctx, EventInput{
		ProjectID: projectID,
		AppID:     appID,
		Kind:      "app_deleted",
		Severity:  SeverityWarn,
		Message:   fmt.Sprintf("app %s deleted", a.Name),
		Meta: map[string]any{
			"cluster_id":            p.ClusterID,
			"namespace":             p.Namespace,
			"ingress_hosts_removed": len(ingHosts),
			"ingress_hosts":         ingHosts,
			"app_name":              a.Name,
		},
	}); err != nil {
		return err
	}
	logging.AppLogger(projectID, appID).Info("app_deleted", fields...)
	return nil
}
