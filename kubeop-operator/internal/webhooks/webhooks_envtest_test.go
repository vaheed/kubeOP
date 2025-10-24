package webhooks

import (
	"context"
	"crypto/tls"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
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

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add app scheme: %v", err)
	}

	env := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("kubeop-operator", "kustomize", "bases", "crds")},
		WebhookInstallOptions: envtest.WebhookInstallOptions{LocalServingHost: "127.0.0.1", LocalServingPort: 0},
	}

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
