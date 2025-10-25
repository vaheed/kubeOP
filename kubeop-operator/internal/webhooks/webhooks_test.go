package webhooks

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/vaheed/kubeOP/kubeop-operator/apis/paas/v1alpha1"
	"github.com/vaheed/kubeOP/kubeop-operator/internal/policy"
	"go.uber.org/zap"
)

func newTestAppWebhook(t *testing.T, objects ...runtime.Object) *AppWebhook {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add app scheme: %v", err)
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()
	return &AppWebhook{client: cli, logger: zap.NewNop().Sugar(), policyLoader: &configMapRegistryPolicyLoader{client: cli, logger: zap.NewNop().Sugar()}}
}

func newTestServiceBindingWebhook(t *testing.T, objects ...runtime.Object) *ServiceBindingWebhook {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add app scheme: %v", err)
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()
	return &ServiceBindingWebhook{client: cli, logger: zap.NewNop().Sugar()}
}

func newTestAppReleaseWebhook(t *testing.T, objects ...runtime.Object) *AppReleaseWebhook {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add app scheme: %v", err)
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()
	return &AppReleaseWebhook{client: cli, logger: zap.NewNop().Sugar()}
}

func newTestBucketWebhook(t *testing.T, objects ...runtime.Object) *BucketWebhook {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add app scheme: %v", err)
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()
	return &BucketWebhook{client: cli, logger: zap.NewNop().Sugar()}
}

func TestAppWebhookRejectsMissingTenantLabel(t *testing.T) {
	webhook := newTestAppWebhook(t)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "team-a",
			Labels: map[string]string{
				labelProject: "proj",
				labelApp:     "app",
			},
		},
	}
	_, err := webhook.ValidateCreate(context.Background(), app)
	if err == nil {
		t.Fatalf("expected error for missing tenant label")
	}
	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected invalid error, got %v", err)
	}
}

func TestAppWebhookRejectsInvalidSemver(t *testing.T) {
	webhook := newTestAppWebhook(t)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-a",
				labelProject: "proj",
				labelApp:     "app",
			},
		},
		Spec: appv1alpha1.AppSpec{Type: appv1alpha1.AppTypeRaw, Version: "not-a-version"},
	}
	if _, err := webhook.ValidateCreate(context.Background(), app); err == nil {
		t.Fatalf("expected invalid semver to be rejected")
	} else if !apierrors.IsInvalid(err) {
		t.Fatalf("expected invalid error, got %v", err)
	}
}

func TestAppWebhookRejectsInvalidSemverRange(t *testing.T) {
	webhook := newTestAppWebhook(t)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-a",
				labelProject: "proj",
				labelApp:     "app",
			},
		},
		Spec: appv1alpha1.AppSpec{Type: appv1alpha1.AppTypeRaw, VersionRange: "1.x"},
	}
	if _, err := webhook.ValidateCreate(context.Background(), app); err == nil {
		t.Fatalf("expected invalid semver range to be rejected")
	} else if !apierrors.IsInvalid(err) {
		t.Fatalf("expected invalid error, got %v", err)
	}
}

func TestAppWebhookRejectsCrossTenantSecretRef(t *testing.T) {
	secret := &appv1alpha1.SecretRef{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shared",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-b",
				labelProject: "proj",
				labelApp:     "app",
			},
		},
	}
	webhook := newTestAppWebhook(t, secret)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-a",
				labelProject: "proj",
				labelApp:     "app",
			},
		},
		Spec: appv1alpha1.AppSpec{
			Type:        appv1alpha1.AppTypeRaw,
			SecretsRefs: []string{"shared"},
		},
	}
	_, err := webhook.ValidateCreate(context.Background(), app)
	if err == nil {
		t.Fatalf("expected cross-tenant validation error")
	}
	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected invalid error, got %v", err)
	}
}

func TestAppWebhookServicePolicyEnforcement(t *testing.T) {
	baseProject := &appv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "proj",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-a",
				labelProject: "proj",
				labelApp:     "app",
			},
		},
		Spec: appv1alpha1.ProjectSpec{
			TenantRef:               "tenant-a",
			NetworkPolicyProfileRef: "restricted",
			Purpose:                 "test",
			Environment:             appv1alpha1.ProjectEnvironmentDev,
			NamespaceName:           "team-a",
		},
	}
	restrictedProfile := &appv1alpha1.NetworkPolicyProfile{
		ObjectMeta: metav1.ObjectMeta{Name: "restricted"},
		Spec: appv1alpha1.NetworkPolicyProfileSpec{
			Presets: []appv1alpha1.NetworkPolicyPreset{appv1alpha1.NetworkPolicyPreset("deny-all")},
		},
	}
	webhook := newTestAppWebhook(t, restrictedProfile, baseProject)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-a",
				labelProject: "proj",
				labelApp:     "app",
			},
		},
		Spec: appv1alpha1.AppSpec{
			Type: appv1alpha1.AppTypeRaw,
			ServiceProfile: &appv1alpha1.AppServiceProfile{
				Type: corev1.ServiceTypeLoadBalancer,
			},
		},
	}
	if _, err := webhook.ValidateCreate(context.Background(), app); err == nil {
		t.Fatalf("expected load balancer to be rejected without policy")
	}

	allowedProfile := restrictedProfile.DeepCopy()
	allowedProfile.Name = "allow-lb"
	allowedProfile.Spec.ServicePolicy = &appv1alpha1.ServiceExposurePolicy{AllowedTypes: []corev1.ServiceType{corev1.ServiceTypeLoadBalancer}}
	allowedProject := baseProject.DeepCopy()
	allowedProject.Spec.NetworkPolicyProfileRef = allowedProfile.Name
	webhook = newTestAppWebhook(t, allowedProfile, allowedProject)
	if _, err := webhook.ValidateCreate(context.Background(), app); err != nil {
		t.Fatalf("expected load balancer to pass when policy allows it, got %v", err)
	}

	allowedProfile.Spec.ServicePolicy.AllowedExternalIPs = []string{"1.2.3.4"}
	webhook = newTestAppWebhook(t, allowedProfile, allowedProject)
	app = app.DeepCopy()
	app.Spec.ServiceProfile.ExternalIPs = []string{"5.6.7.8"}
	if _, err := webhook.ValidateCreate(context.Background(), app); err == nil {
		t.Fatalf("expected external IP outside allowlist to be rejected")
	}
}

func TestAppWebhookRegistryPolicyEnforced(t *testing.T) {
	policyConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      policy.ConfigMapName,
			Namespace: "team-a",
		},
		Data: map[string]string{
			"allowedRegistries":   "ghcr.io",
			"allowedRepositories": "ghcr.io/allowed/app\nghcr.io/allowed/chart",
		},
	}
	project := &appv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "proj",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-a",
				labelProject: "proj",
			},
		},
		Spec: appv1alpha1.ProjectSpec{
			TenantRef:     "tenant-a",
			NamespaceName: "team-a",
		},
	}
	webhook := newTestAppWebhook(t, policyConfig, project)

	baseLabels := map[string]string{labelTenant: "tenant-a", labelProject: "proj", labelApp: "app"}
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "image-app",
			Namespace: "team-a",
			Labels:    baseLabels,
		},
		Spec: appv1alpha1.AppSpec{Type: appv1alpha1.AppTypeRaw, Image: "ghcr.io/allowed/app:1.0.0"},
	}
	if _, err := webhook.ValidateCreate(context.Background(), app); err != nil {
		t.Fatalf("expected allowlisted image to pass, got %v", err)
	}

	app = app.DeepCopy()
	app.Spec.Image = "docker.io/library/nginx:latest"
	if _, err := webhook.ValidateCreate(context.Background(), app); err == nil {
		t.Fatalf("expected non-allowlisted registry to be rejected")
	} else if !apierrors.IsInvalid(err) {
		t.Fatalf("expected invalid error, got %v", err)
	}

	helmApp := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "helm-app",
			Namespace: "team-a",
			Labels:    baseLabels,
		},
		Spec: appv1alpha1.AppSpec{Type: appv1alpha1.AppTypeHelmOCI, Source: &appv1alpha1.AppSource{URL: "oci://ghcr.io/allowed/chart"}},
	}
	if _, err := webhook.ValidateCreate(context.Background(), helmApp); err != nil {
		t.Fatalf("expected allowlisted OCI source to pass, got %v", err)
	}

	helmApp = helmApp.DeepCopy()
	helmApp.Spec.Source.URL = "oci://quay.io/other/chart"
	if _, err := webhook.ValidateCreate(context.Background(), helmApp); err == nil {
		t.Fatalf("expected non-allowlisted OCI registry to be rejected")
	} else if !apierrors.IsInvalid(err) {
		t.Fatalf("expected invalid error, got %v", err)
	}
}

func TestJobWebhookRejectsHostNetwork(t *testing.T) {
	job := &appv1alpha1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "batch",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-a",
				labelProject: "proj",
				labelApp:     "app",
			},
		},
		Spec: appv1alpha1.JobSpec{
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{HostNetwork: true}},
		},
	}
	webhook := &JobWebhook{logger: zap.NewNop().Sugar()}
	_, err := webhook.ValidateCreate(context.Background(), job)
	if err == nil {
		t.Fatalf("expected hostNetwork to be rejected")
	}
	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected invalid error, got %v", err)
	}
}

func TestServiceBindingWebhookRejectsCrossTenantProvider(t *testing.T) {
	provider := &appv1alpha1.DatabaseInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "db",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-b",
				labelProject: "proj",
				labelApp:     "app",
			},
		},
	}
	binding := &appv1alpha1.ServiceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bind",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-a",
				labelProject: "proj",
				labelApp:     "app",
			},
		},
		Spec: appv1alpha1.ServiceBindingSpec{
			Consumer: appv1alpha1.ServiceBindingConsumer{Type: appv1alpha1.ServiceBindingConsumerTypeApp, Name: "demo"},
			Provider: appv1alpha1.ServiceBindingProvider{Type: appv1alpha1.ServiceBindingProviderTypeDatabase, Name: "db"},
			InjectAs: appv1alpha1.ServiceBindingInjectionTypeEnv,
		},
	}
	consumer := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-a",
				labelProject: "proj",
				labelApp:     "app",
			},
		},
		Spec: appv1alpha1.AppSpec{Type: appv1alpha1.AppTypeRaw},
	}
	webhook := newTestServiceBindingWebhook(t, provider, consumer)
	if _, err := webhook.ValidateCreate(context.Background(), binding); err == nil {
		t.Fatalf("expected cross-tenant provider to be rejected")
	}
}

func TestAppReleaseWebhookRejectsSpecMutation(t *testing.T) {
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-app",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-a",
				labelProject: "proj",
				labelApp:     "app",
			},
		},
	}
	oldRelease := &appv1alpha1.AppRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-release",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-a",
				labelProject: "proj",
				labelApp:     "app",
			},
		},
		Spec: appv1alpha1.AppReleaseSpec{
			AppRef:             "demo-app",
			Version:            "1.0.0",
			Digest:             "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			RenderedConfigHash: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		},
	}
	newRelease := oldRelease.DeepCopy()
	newRelease.Spec.RenderedConfigHash = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

	webhook := newTestAppReleaseWebhook(t, app)
	if _, err := webhook.ValidateUpdate(context.Background(), oldRelease, newRelease); err == nil {
		t.Fatalf("expected AppRelease spec mutation to be rejected")
	} else if !apierrors.IsInvalid(err) {
		t.Fatalf("expected invalid error, got %v", err)
	}
}

func TestAppReleaseWebhookValidatesDigests(t *testing.T) {
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-app",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-a",
				labelProject: "proj",
				labelApp:     "app",
			},
		},
	}
	release := &appv1alpha1.AppRelease{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo-release",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-a",
				labelProject: "proj",
				labelApp:     "app",
			},
		},
		Spec: appv1alpha1.AppReleaseSpec{
			AppRef:             "demo-app",
			Version:            "1.0.0",
			Digest:             "not-a-digest",
			RenderedConfigHash: "gggg",
		},
	}
	webhook := newTestAppReleaseWebhook(t, app)
	if _, err := webhook.ValidateCreate(context.Background(), release); err == nil {
		t.Fatalf("expected invalid digest and hash to be rejected")
	} else if !apierrors.IsInvalid(err) {
		t.Fatalf("expected invalid error, got %v", err)
	}
}

func TestBucketWebhookRejectsCrossTenantPolicy(t *testing.T) {
	policy := &appv1alpha1.BucketPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "policy",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-b",
				labelProject: "proj",
				labelApp:     "app",
			},
		},
		Spec: appv1alpha1.BucketPolicySpec{Statements: []appv1alpha1.BucketPolicyStatement{{Effect: "Allow", Actions: []string{"Get"}, Principals: []string{"app"}}}},
	}
	bucket := &appv1alpha1.Bucket{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bucket",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-a",
				labelProject: "proj",
				labelApp:     "app",
			},
		},
		Spec: appv1alpha1.BucketSpec{Provider: appv1alpha1.BucketProvider("s3"), PolicyRefs: []string{"policy"}},
	}
	webhook := newTestBucketWebhook(t, policy)
	if _, err := webhook.ValidateCreate(context.Background(), bucket); err == nil {
		t.Fatalf("expected cross-tenant bucket policy to be rejected")
	}
}

func TestJobWebhookRequiresRunAsRootJustification(t *testing.T) {
	runAsUser := int64(0)
	job := &appv1alpha1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "root-job",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-a",
				labelProject: "proj",
				labelApp:     "app",
			},
		},
		Spec: appv1alpha1.JobSpec{
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name:  "runner",
				Image: "busybox",
				SecurityContext: &corev1.SecurityContext{
					RunAsUser: &runAsUser,
				},
			}}}},
		},
	}
	webhook := &JobWebhook{logger: zap.NewNop().Sugar()}
	_, err := webhook.ValidateCreate(context.Background(), job)
	if err == nil {
		t.Fatalf("expected runAsRoot to require justification")
	}
	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected invalid error, got %v", err)
	}

	job = job.DeepCopy()
	job.Annotations = map[string]string{annotationRunAsRootJustification: "needs root for init"}
	if _, err := webhook.ValidateCreate(context.Background(), job); err != nil {
		t.Fatalf("expected justification to allow runAsRoot, got %v", err)
	}
}
