package webhooks

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	admv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	appv1alpha1 "github.com/vaheed/kubeOP/kubeop-operator/apis/paas/v1alpha1"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestEnvtestAppWebhookMissingTenant(t *testing.T) {
	runWebhookEnvtest(t, func(ctx context.Context, c client.Client) {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
		if err := c.Create(ctx, ns); err != nil {
			t.Fatalf("create namespace: %v", err)
		}
		app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "team-a",
			Labels: map[string]string{
				labelProject: "proj",
				labelApp:     "app",
			},
		}, Spec: appv1alpha1.AppSpec{Type: appv1alpha1.AppTypeRaw}}
		if err := c.Create(ctx, app); err == nil {
			t.Fatalf("expected validation error")
		}
	})
}

func TestEnvtestAppWebhookCrossTenantSecret(t *testing.T) {
	runWebhookEnvtest(t, func(ctx context.Context, c client.Client) {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
		if err := c.Create(ctx, ns); err != nil {
			t.Fatalf("create namespace: %v", err)
		}
		secretRef := &appv1alpha1.SecretRef{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shared",
				Namespace: "team-a",
				Labels: map[string]string{
					labelTenant:  "tenant-b",
					labelProject: "proj",
					labelApp:     "app",
				},
			},
			Spec: appv1alpha1.SecretRefSpec{Data: appv1alpha1.SecretRefData{Type: appv1alpha1.SecretRefDataType("inline"), Inline: map[string]string{"key": "value"}}},
		}
		if err := c.Create(ctx, secretRef); err != nil {
			t.Fatalf("create secretref: %v", err)
		}
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
		if err := c.Create(ctx, app); err == nil {
			t.Fatalf("expected cross-tenant secret to be rejected")
		}
	})
}

func TestEnvtestAppWebhookServicePolicy(t *testing.T) {
	runWebhookEnvtest(t, func(ctx context.Context, c client.Client) {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a"}}
		if err := c.Create(ctx, ns); err != nil {
			t.Fatalf("create namespace: %v", err)
		}

		profile := &appv1alpha1.NetworkPolicyProfile{
			ObjectMeta: metav1.ObjectMeta{Name: "restricted"},
			Spec:       appv1alpha1.NetworkPolicyProfileSpec{Presets: []appv1alpha1.NetworkPolicyPreset{appv1alpha1.NetworkPolicyPreset("deny-all")}},
		}
		if err := c.Create(ctx, profile); err != nil {
			t.Fatalf("create profile: %v", err)
		}

		project := &appv1alpha1.Project{
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
				Purpose:                 "test",
				Environment:             appv1alpha1.ProjectEnvironmentDev,
				NamespaceName:           "team-a",
				NetworkPolicyProfileRef: "restricted",
			},
		}
		if err := c.Create(ctx, project); err != nil {
			t.Fatalf("create project: %v", err)
		}

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
			Spec: appv1alpha1.AppSpec{Type: appv1alpha1.AppTypeRaw, ServiceProfile: &appv1alpha1.AppServiceProfile{Type: corev1.ServiceTypeLoadBalancer}},
		}
		if err := c.Create(ctx, app); err == nil {
			t.Fatalf("expected load balancer to be rejected without service policy")
		}

		if err := c.Get(ctx, types.NamespacedName{Name: "restricted"}, profile); err != nil {
			t.Fatalf("get profile: %v", err)
		}
		profile.Spec.ServicePolicy = &appv1alpha1.ServiceExposurePolicy{AllowedTypes: []corev1.ServiceType{corev1.ServiceTypeLoadBalancer}}
		if err := c.Update(ctx, profile); err != nil {
			t.Fatalf("update profile: %v", err)
		}

		app.ObjectMeta = metav1.ObjectMeta{
			Name:      "demo-allowed",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-a",
				labelProject: "proj",
				labelApp:     "app",
			},
		}
		if err := c.Create(ctx, app); err != nil {
			t.Fatalf("expected load balancer to be allowed after policy update: %v", err)
		}
	})
}

func runWebhookEnvtest(t *testing.T, fn func(context.Context, client.Client)) {
	t.Helper()

	scheme := apiruntime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add app scheme: %v", err)
	}

	assetsDir := resolveEnvtestAssetsDir(t)

	env := &envtest.Environment{
		CRDInstallOptions:        envtest.CRDInstallOptions{CRDs: buildEnvtestCRDs()},
		WebhookInstallOptions:    envtest.WebhookInstallOptions{LocalServingHost: "127.0.0.1", LocalServingPort: webhookListenPort},
		BinaryAssetsDirectory:    assetsDir,
		DownloadBinaryAssets:     true,
		ErrorIfCRDPathMissing:    true,
		ControlPlaneStartTimeout: 2 * time.Minute,
		ControlPlaneStopTimeout:  time.Minute,
		AttachControlPlaneOutput: testing.Verbose(),
	}
	if env.ControlPlane.APIServer == nil {
		env.ControlPlane.APIServer = &envtest.APIServer{}
	}
	apiServerArgs := env.ControlPlane.APIServer.Configure()
	apiServerArgs.Set("advertise-address", "127.0.0.1")
	apiServerArgs.Set("bind-address", "127.0.0.1")

	cfg, err := env.Start()
	if err != nil {
		t.Fatalf("start envtest: %v", err)
	}
	defer func() {
		if stopErr := env.Stop(); stopErr != nil {
			t.Fatalf("stop envtest: %v", stopErr)
		}
	}()

	zapLogger := buildTestLogger()
	defer func() { _ = zapLogger.Sync() }()
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    env.WebhookInstallOptions.LocalServingHost,
			Port:    env.WebhookInstallOptions.LocalServingPort,
			CertDir: env.WebhookInstallOptions.LocalServingCertDir,
		}),
	})
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	if err := Setup(mgr, zapLogger.Sugar()); err != nil {
		t.Fatalf("setup webhooks: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = mgr.Start(ctx)
	}()

	addr := fmt.Sprintf("%s:%d", env.WebhookInstallOptions.LocalServingHost, env.WebhookInstallOptions.LocalServingPort)
	if err := wait.PollImmediate(100*time.Millisecond, 10*time.Second, func() (bool, error) {
		conn, err := tls.Dial("tcp", addr, &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			return false, nil
		}
		_ = conn.Close()
		return true, nil
	}); err != nil {
		t.Fatalf("webhook server not ready: %v", err)
	}

	kubeClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("build client: %v", err)
	}
	if err := ensureAppWebhookConfigurations(ctx, cfg, env.WebhookInstallOptions, kubeClient); err != nil {
		t.Fatalf("install webhook configurations: %v", err)
	}

	fn(ctx, kubeClient)
}

func buildTestLogger() *zap.Logger {
	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(zapcore.ErrorLevel),
		Development:      true,
		Encoding:         "console",
		EncoderConfig:    zap.NewDevelopmentEncoderConfig(),
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
	logger, err := cfg.Build()
	if err != nil {
		panic(err)
	}
	return logger
}

var (
	envtestAssetsOnce sync.Once
	envtestAssetsDir  string
	envtestAssetsErr  error
)

func resolveEnvtestAssetsDir(t *testing.T) string {
	t.Helper()

	envtestAssetsOnce.Do(func() {
		dir, err := envtest.SetupEnvtestDefaultBinaryAssetsDirectory()
		if err != nil {
			envtestAssetsErr = fmt.Errorf("resolve envtest assets directory: %w", err)
			return
		}
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			envtestAssetsErr = fmt.Errorf("create envtest assets directory %q: %w", dir, mkErr)
			return
		}
		envtestAssetsDir = dir
	})

	if envtestAssetsErr != nil {
		t.Logf("envtest assets directory fallback: %v", envtestAssetsErr)
		return ""
	}

	return envtestAssetsDir
}

func ensureAppWebhookConfigurations(ctx context.Context, cfg *rest.Config, opts envtest.WebhookInstallOptions, kubeClient client.Client) error {
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("build kubernetes clientset: %w", err)
	}
	if len(opts.LocalServingCAData) == 0 {
		return fmt.Errorf("webhook CA bundle is empty")
	}
	if opts.LocalServingPort == 0 {
		return fmt.Errorf("webhook listen port is not configured")
	}
	host := opts.LocalServingHost
	if strings.TrimSpace(host) == "" {
		host = "127.0.0.1"
	}
	webhookURL := fmt.Sprintf("https://%s:%d%%s", host, opts.LocalServingPort)
	scope := admv1.NamespacedScope
	sideEffectsNone := admv1.SideEffectClassNone
	failurePolicy := admv1.Fail
	mutating := &admv1.MutatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "kubeop-app-mutating"},
		Webhooks: []admv1.MutatingWebhook{
			{
				Name:                    "mapp.paas.kubeop.io",
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				ClientConfig: admv1.WebhookClientConfig{
					URL:      ptr(fmt.Sprintf(webhookURL, appMutatingWebhookPath)),
					CABundle: opts.LocalServingCAData,
				},
				FailurePolicy: &failurePolicy,
				SideEffects:   &sideEffectsNone,
				MatchPolicy:   ptr(admv1.Equivalent),
				Rules: []admv1.RuleWithOperations{
					{
						Operations: []admv1.OperationType{admv1.Create, admv1.Update},
						Rule: admv1.Rule{
							APIGroups:   []string{paasGroup},
							APIVersions: []string{appv1alpha1.GroupVersion.Version},
							Resources:   []string{"apps"},
							Scope:       &scope,
						},
					},
				},
			},
		},
	}
	if _, err := clientset.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(ctx, mutating, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create app mutating webhook configuration: %w", err)
	}

	validating := &admv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{Name: "kubeop-app-validating"},
		Webhooks: []admv1.ValidatingWebhook{
			{
				Name:                    "vapp.paas.kubeop.io",
				AdmissionReviewVersions: []string{"v1", "v1beta1"},
				ClientConfig: admv1.WebhookClientConfig{
					URL:      ptr(fmt.Sprintf(webhookURL, appValidatingWebhookPath)),
					CABundle: opts.LocalServingCAData,
				},
				FailurePolicy: &failurePolicy,
				SideEffects:   &sideEffectsNone,
				MatchPolicy:   ptr(admv1.Equivalent),
				Rules: []admv1.RuleWithOperations{
					{
						Operations: []admv1.OperationType{admv1.Create, admv1.Update},
						Rule: admv1.Rule{
							APIGroups:   []string{paasGroup},
							APIVersions: []string{appv1alpha1.GroupVersion.Version},
							Resources:   []string{"apps"},
							Scope:       &scope,
						},
					},
				},
			},
		},
	}
	if _, err := clientset.AdmissionregistrationV1().ValidatingWebhookConfigurations().Create(ctx, validating, metav1.CreateOptions{}); err != nil {
		return fmt.Errorf("create app validating webhook configuration: %w", err)
	}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "webhook-probe"}}
	if _, err := clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create probe namespace: %w", err)
	}

	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := wait.PollImmediateWithContext(probeCtx, 100*time.Millisecond, 5*time.Second, func(ctx context.Context) (bool, error) {
		probeApp := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{
			Name:      "probe",
			Namespace: ns.Name,
			Labels: map[string]string{
				labelProject: "probe",
				labelApp:     "probe",
			},
		}, Spec: appv1alpha1.AppSpec{Type: appv1alpha1.AppTypeRaw}}
		err := kubeClient.Create(ctx, probeApp)
		if err == nil {
			// webhook not yet enforced; clean up and retry
			_ = kubeClient.Delete(ctx, probeApp)
			return false, nil
		}
		if apierrors.IsInvalid(err) || apierrors.IsForbidden(err) {
			return true, nil
		}
		if apierrors.IsAlreadyExists(err) {
			return true, nil
		}
		// Retry on transient errors such as missing namespace propagation
		return false, nil
	}); err != nil {
		return fmt.Errorf("wait for app webhook readiness: %w", err)
	}

	return nil
}

func buildEnvtestCRDs() []*apiextensionsv1.CustomResourceDefinition {
	return []*apiextensionsv1.CustomResourceDefinition{
		minimalNamespacedCRD("App", "apps"),
		minimalNamespacedCRD("Project", "projects"),
		minimalNamespacedCRD("SecretRef", "secretrefs"),
		minimalClusterCRD("NetworkPolicyProfile", "networkpolicyprofiles"),
	}
}

func minimalNamespacedCRD(kind, plural string) *apiextensionsv1.CustomResourceDefinition {
	return minimalCRD(kind, plural, apiextensionsv1.NamespaceScoped)
}

func minimalClusterCRD(kind, plural string) *apiextensionsv1.CustomResourceDefinition {
	return minimalCRD(kind, plural, apiextensionsv1.ClusterScoped)
}

func minimalCRD(kind, plural string, scope apiextensionsv1.ResourceScope) *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s.%s", plural, paasGroup),
		},
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Group: paasGroup,
			Scope: scope,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:     kind,
				Plural:   plural,
				Singular: strings.ToLower(kind),
			},
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    "v1alpha1",
					Served:  true,
					Storage: true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{Type: "object", XPreserveUnknownFields: ptr(true)},
					},
				},
			},
		},
	}
}

const paasGroup = "paas.kubeop.io"
const webhookListenPort = 9443
const appMutatingWebhookPath = "/mutate-paas-kubeop-io-v1alpha1-app"
const appValidatingWebhookPath = "/validate-paas-kubeop-io-v1alpha1-app"

func ptr[T any](v T) *T {
	return &v
}
