package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"go.uber.org/zap"

	"kubeop/internal/config"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *Service) ensureOperatorDeployment(ctx context.Context, clusterID, kubeconfig string) error {
	kubeconfig = strings.TrimSpace(kubeconfig)
	if kubeconfig == "" {
		return errors.New("kubeconfig required")
	}
	if s.km == nil {
		return errors.New("kube manager not configured")
	}
	logger := s.logger
	if logger == nil {
		logger = zap.NewNop()
	}
	log := logger.With(zap.String("cluster_id", clusterID))
	log.Info("ensuring kubeop-operator deployment", zap.String("namespace", s.cfg.OperatorNamespace))
	loader := func(context.Context) ([]byte, error) {
		return []byte(kubeconfig), nil
	}
	client, err := s.km.GetOrCreate(ctx, clusterID, loader)
	if err != nil {
		return fmt.Errorf("build cluster client: %w", err)
	}
	resources, err := buildOperatorResources(s.cfg)
	if err != nil {
		return err
	}
	for _, obj := range resources {
		if err := createOrUpdate(ctx, client, obj); err != nil {
			kind := obj.GetObjectKind().GroupVersionKind().Kind
			return fmt.Errorf("ensure %s/%s: %w", kind, obj.GetName(), err)
		}
	}
	log.Info("kubeop-operator deployment ensured", zap.String("namespace", s.cfg.OperatorNamespace))
	return nil
}

func buildOperatorResources(cfg *config.Config) ([]crclient.Object, error) {
	pullPolicy, err := parsePullPolicy(cfg.OperatorImagePullPolicy)
	if err != nil {
		return nil, err
	}
	baseLabels := map[string]string{
		"app.kubernetes.io/name":       cfg.OperatorDeploymentName,
		"app.kubernetes.io/instance":   cfg.OperatorDeploymentName,
		"app.kubernetes.io/component":  "operator",
		"app.kubernetes.io/part-of":    "kubeop",
		"app.kubernetes.io/managed-by": "kubeop-api",
		"kubeop.io/managed":            "true",
	}

	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: cfg.OperatorNamespace, Labels: cloneLabels(baseLabels)}}
	namespace.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("Namespace"))

	sa := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: cfg.OperatorServiceAccount, Namespace: cfg.OperatorNamespace, Labels: cloneLabels(baseLabels)}}
	sa.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("ServiceAccount"))

	clusterRole := &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: cfg.OperatorDeploymentName, Labels: cloneLabels(baseLabels)}}
	clusterRole.SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("ClusterRole"))
	clusterRole.Rules = []rbacv1.PolicyRule{
		{
			APIGroups: []string{"app.kubeop.io"},
			Resources: []string{"apps"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
		},
		{
			APIGroups: []string{"app.kubeop.io"},
			Resources: []string{"apps/status"},
			Verbs:     []string{"get", "patch", "update"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"deployments"},
			Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
		},
		{
			APIGroups: []string{"apps"},
			Resources: []string{"replicasets"},
			Verbs:     []string{"get", "list", "watch"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"pods"},
			Verbs:     []string{"get", "list", "watch"},
		},
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprintf("%s-binding", cfg.OperatorDeploymentName),
			Labels: cloneLabels(baseLabels),
		},
		RoleRef: rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: clusterRole.Name},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      cfg.OperatorServiceAccount,
			Namespace: cfg.OperatorNamespace,
		}},
	}
	clusterRoleBinding.SetGroupVersionKind(rbacv1.SchemeGroupVersion.WithKind("ClusterRoleBinding"))

	replicas := int32(1)
	args := []string{}
	if cfg.OperatorLeaderElection {
		args = append(args, "--leader-elect")
	}
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.OperatorDeploymentName,
			Namespace: cfg.OperatorNamespace,
			Labels:    cloneLabels(baseLabels),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: cloneLabels(baseLabels)},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: cloneLabels(baseLabels)},
				Spec: corev1.PodSpec{
					ServiceAccountName: cfg.OperatorServiceAccount,
					Containers: []corev1.Container{{
						Name:            "manager",
						Image:           cfg.OperatorImage,
						ImagePullPolicy: pullPolicy,
						Args:            args,
						Ports: []corev1.ContainerPort{{
							Name:          "metrics",
							ContainerPort: 8080,
							Protocol:      corev1.ProtocolTCP,
						}, {
							Name:          "probes",
							ContainerPort: 8081,
							Protocol:      corev1.ProtocolTCP,
						}},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler:        corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/readyz", Port: intstr.FromInt(8081)}},
							PeriodSeconds:       15,
							FailureThreshold:    3,
							SuccessThreshold:    1,
							InitialDelaySeconds: 10,
							TimeoutSeconds:      5,
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler:        corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{Path: "/healthz", Port: intstr.FromInt(8081)}},
							PeriodSeconds:       20,
							FailureThreshold:    3,
							SuccessThreshold:    1,
							InitialDelaySeconds: 15,
							TimeoutSeconds:      5,
						},
					}},
				},
			},
		},
	}
	deployment.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))

	crd := buildAppCRD(baseLabels)

	return []crclient.Object{namespace, sa, clusterRole, clusterRoleBinding, crd, deployment}, nil
}

func createOrUpdate(ctx context.Context, c crclient.Client, obj crclient.Object) error {
	key := crclient.ObjectKey{Name: obj.GetName(), Namespace: obj.GetNamespace()}
	current := obj.DeepCopyObject().(crclient.Object)
	if err := c.Get(ctx, key, current); err != nil {
		if apierrors.IsNotFound(err) {
			return c.Create(ctx, obj)
		}
		return err
	}
	obj.SetResourceVersion(current.GetResourceVersion())
	return c.Update(ctx, obj)
}

func parsePullPolicy(policy string) (corev1.PullPolicy, error) {
	p := corev1.PullPolicy(strings.TrimSpace(policy))
	if p == "" {
		return corev1.PullIfNotPresent, nil
	}
	switch p {
	case corev1.PullAlways, corev1.PullIfNotPresent, corev1.PullNever:
		return p, nil
	default:
		return "", fmt.Errorf("invalid operator image pull policy %q", policy)
	}
}

func cloneLabels(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func buildAppCRD(baseLabels map[string]string) *apiextensionsv1.CustomResourceDefinition {
	crd := &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "apps.app.kubeop.io",
			Labels: cloneLabels(baseLabels),
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: "app.kubeop.io",
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Plural:     "apps",
				Singular:   "app",
				Kind:       "App",
				ShortNames: []string{"kapp"},
				Categories: []string{"kubeop"},
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{{
				Name:    "v1alpha1",
				Served:  true,
				Storage: true,
				Schema:  &apiextensionsv1.CustomResourceValidation{OpenAPIV3Schema: buildAppSchema()},
				Subresources: &apiextensionsv1.CustomResourceSubresources{
					Status: &apiextensionsv1.CustomResourceSubresourceStatus{},
				},
				AdditionalPrinterColumns: []apiextensionsv1.CustomResourceColumnDefinition{
					{Name: "Image", Type: "string", JSONPath: ".spec.image"},
					{Name: "Replicas", Type: "integer", JSONPath: ".spec.replicas"},
					{Name: "Ready", Type: "string", JSONPath: ".status.conditions[?(@.type==\"Ready\")].status"},
				},
			}},
		},
	}
	crd.SetGroupVersionKind(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"))
	return crd
}

func buildAppSchema() *apiextensionsv1.JSONSchemaProps {
	return &apiextensionsv1.JSONSchemaProps{
		Type: "object",
		Properties: map[string]apiextensionsv1.JSONSchemaProps{
			"spec": {
				Type: "object",
				Properties: map[string]apiextensionsv1.JSONSchemaProps{
					"image":    {Type: "string"},
					"replicas": {Type: "integer", Format: "int32"},
					"hosts": {
						Type:  "array",
						Items: &apiextensionsv1.JSONSchemaPropsOrArray{Schema: &apiextensionsv1.JSONSchemaProps{Type: "string"}},
					},
				},
			},
			"status": {
				Type: "object",
				Properties: map[string]apiextensionsv1.JSONSchemaProps{
					"observedGeneration": {Type: "integer", Format: "int64"},
					"availableReplicas":  {Type: "integer", Format: "int32"},
					"conditions": {
						Type: "array",
						Items: &apiextensionsv1.JSONSchemaPropsOrArray{Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"type":               {Type: "string"},
								"status":             {Type: "string"},
								"reason":             {Type: "string"},
								"message":            {Type: "string"},
								"lastTransitionTime": {Type: "string", Format: "date-time"},
							},
						}},
					},
				},
			},
		},
	}
}
