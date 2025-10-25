package controllers

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	appv1alpha1 "github.com/vaheed/kubeOP/kubeop-operator/apis/paas/v1alpha1"
	"github.com/vaheed/kubeOP/kubeop-operator/internal/metrics"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestUsageWriterReconcilerAggregatesUsage(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add app scheme: %v", err)
	}

	tenant := &appv1alpha1.Tenant{ObjectMeta: metav1.ObjectMeta{Name: "acme"}}
	project := &appv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "payments",
			Namespace: "acme-dev",
		},
		Spec: appv1alpha1.ProjectSpec{
			TenantRef:     "acme",
			Purpose:       "billing",
			Environment:   appv1alpha1.ProjectEnvironmentProd,
			NamespaceName: "acme-dev",
		},
	}
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "checkout",
			Namespace: "acme-dev",
		},
		Spec: appv1alpha1.AppSpec{Type: appv1alpha1.AppTypeRaw},
	}

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&appv1alpha1.Tenant{}, &appv1alpha1.Project{}, &appv1alpha1.BillingUsage{}).
		WithRuntimeObjects(tenant, project, app).
		Build()

	fakeProvider := metrics.NewFakeProvider()
	window := time.Date(2026, 2, 20, 10, 4, 0, 0, time.UTC)
	fakeProvider.SetUsage(window, []metrics.UsageSample{
		{
			Tenant:           "acme",
			ProjectNamespace: "acme-dev",
			Project:          "payments",
			AppNamespace:     "acme-dev",
			App:              "checkout",
			CPU:              resource.MustParse("250m"),
			Memory:           resource.MustParse("512Mi"),
			Storage:          resource.MustParse("10Gi"),
			Egress:           resource.MustParse("1Gi"),
			LBHours:          resource.MustParse("2"),
		},
	})

	reconciler := &UsageWriterReconciler{
		Client: client,
		Scheme: scheme,
		Logger: zap.NewNop().Sugar(),
		Source: fakeProvider,
		Clock:  &fakeClock{now: window},
	}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	var updatedTenant appv1alpha1.Tenant
	if err := client.Get(context.Background(), types.NamespacedName{Name: "acme"}, &updatedTenant); err != nil {
		t.Fatalf("get tenant: %v", err)
	}
	if updatedTenant.Status.Usage == nil {
		t.Fatalf("tenant usage not populated")
	}
	if got := updatedTenant.Status.Usage.CPU.String(); got != "250m" {
		t.Fatalf("expected tenant CPU 250m, got %s", got)
	}
	if cond := apimeta.FindStatusCondition(updatedTenant.Status.Conditions, "Ready"); cond == nil || cond.Status != metav1.ConditionTrue {
		t.Fatalf("expected tenant Ready condition true, got %+v", cond)
	}

	var updatedProject appv1alpha1.Project
	if err := client.Get(context.Background(), types.NamespacedName{Name: "payments", Namespace: "acme-dev"}, &updatedProject); err != nil {
		t.Fatalf("get project: %v", err)
	}
	if updatedProject.Status.Usage == nil {
		t.Fatalf("project usage not populated")
	}
	if got := updatedProject.Status.Usage.Memory.String(); got != "512Mi" {
		t.Fatalf("expected project memory 512Mi, got %s", got)
	}

	var usageList appv1alpha1.BillingUsageList
	if err := client.List(context.Background(), &usageList); err != nil {
		t.Fatalf("list billing usage: %v", err)
	}
	if len(usageList.Items) != 3 {
		t.Fatalf("expected three billing usage records, got %d", len(usageList.Items))
	}
	for _, usage := range usageList.Items {
		if usage.Spec.Window != window.UTC().Truncate(time.Hour).Format(usageWindowFormat) {
			t.Fatalf("unexpected window %s", usage.Spec.Window)
		}
		if usage.Spec.Meters["cpu"] == "" {
			t.Fatalf("usage record missing cpu meter")
		}
	}
}

func TestUsageWriterReconcilerEnvtestWindowing(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add app scheme: %v", err)
	}

	assets := resolveUsageEnvtestAssetsDir(t)
	env := &envtest.Environment{
		CRDInstallOptions:        envtest.CRDInstallOptions{CRDs: buildUsageEnvtestCRDs()},
		BinaryAssetsDirectory:    assets,
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
	t.Cleanup(func() {
		if stopErr := env.Stop(); stopErr != nil {
			t.Fatalf("stop envtest: %v", stopErr)
		}
	})

	kubeClient, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		t.Fatalf("build client: %v", err)
	}

	ctx := context.Background()
	namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "acme"}}
	tenant := &appv1alpha1.Tenant{ObjectMeta: metav1.ObjectMeta{Name: "acme"}}
	project := &appv1alpha1.Project{
		ObjectMeta: metav1.ObjectMeta{Name: "payments", Namespace: "acme"},
		Spec: appv1alpha1.ProjectSpec{
			TenantRef:     "acme",
			Purpose:       "billing",
			Environment:   appv1alpha1.ProjectEnvironmentProd,
			NamespaceName: "acme",
		},
	}
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "checkout", Namespace: "acme"},
		Spec:       appv1alpha1.AppSpec{Type: appv1alpha1.AppTypeRaw},
	}
	for _, obj := range []client.Object{namespace, tenant, project, app} {
		if err := kubeClient.Create(ctx, obj); err != nil {
			t.Fatalf("create %s: %v", obj.GetName(), err)
		}
	}

	fakeProvider := metrics.NewFakeProvider()
	firstWindow := time.Date(2026, 2, 20, 9, 0, 0, 0, time.UTC)
	secondWindow := firstWindow.Add(time.Hour)
	fakeProvider.SetUsage(firstWindow, []metrics.UsageSample{{
		Tenant:           "acme",
		ProjectNamespace: "acme",
		Project:          "payments",
		AppNamespace:     "acme",
		App:              "checkout",
		CPU:              resource.MustParse("100m"),
		Memory:           resource.MustParse("256Mi"),
	}})
	fakeProvider.SetUsage(secondWindow, []metrics.UsageSample{{
		Tenant:           "acme",
		ProjectNamespace: "acme",
		Project:          "payments",
		AppNamespace:     "acme",
		App:              "checkout",
		CPU:              resource.MustParse("200m"),
		Memory:           resource.MustParse("512Mi"),
	}})

	clock := &fakeClock{now: firstWindow}
	reconciler := &UsageWriterReconciler{
		Client: kubeClient,
		Scheme: scheme,
		Logger: zap.NewNop().Sugar(),
		Source: fakeProvider,
		Clock:  clock,
	}

	if _, err := reconciler.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}

	var usageList appv1alpha1.BillingUsageList
	if err := kubeClient.List(ctx, &usageList); err != nil {
		t.Fatalf("list billing usage: %v", err)
	}
	if len(usageList.Items) != 3 {
		t.Fatalf("expected three billing usage objects for first window, got %d", len(usageList.Items))
	}

	clock.now = secondWindow
	if _, err := reconciler.Reconcile(ctx, ctrl.Request{}); err != nil {
		t.Fatalf("third reconcile: %v", err)
	}

	usageList = appv1alpha1.BillingUsageList{}
	if err := kubeClient.List(ctx, &usageList); err != nil {
		t.Fatalf("list billing usage after second window: %v", err)
	}
	windowCount := make(map[string]int)
	for _, usage := range usageList.Items {
		windowCount[usage.Spec.Window]++
	}
	if windowCount[firstWindow.Format(usageWindowFormat)] != 3 {
		t.Fatalf("expected three records for first window, got %d", windowCount[firstWindow.Format(usageWindowFormat)])
	}
	if windowCount[secondWindow.Format(usageWindowFormat)] != 3 {
		t.Fatalf("expected three records for second window, got %d", windowCount[secondWindow.Format(usageWindowFormat)])
	}
}

type fakeClock struct {
	now time.Time
}

func (f *fakeClock) Now() time.Time { return f.now }

var (
	usageEnvtestAssetsOnce sync.Once
	usageEnvtestAssetsDir  string
	usageEnvtestAssetsErr  error
)

func resolveUsageEnvtestAssetsDir(t *testing.T) string {
	t.Helper()
	usageEnvtestAssetsOnce.Do(func() {
		dir, err := envtest.SetupEnvtestDefaultBinaryAssetsDirectory()
		if err != nil {
			usageEnvtestAssetsErr = fmt.Errorf("resolve envtest assets directory: %w", err)
			return
		}
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			usageEnvtestAssetsErr = fmt.Errorf("create envtest assets directory %q: %w", dir, mkErr)
			return
		}
		usageEnvtestAssetsDir = dir
	})
	if usageEnvtestAssetsErr != nil {
		t.Logf("using default envtest assets directory: %v", usageEnvtestAssetsErr)
		return ""
	}
	return usageEnvtestAssetsDir
}

func buildUsageEnvtestCRDs() []*apiextensionsv1.CustomResourceDefinition {
	return []*apiextensionsv1.CustomResourceDefinition{
		minimalNamespacedCRD("App", "apps"),
		minimalNamespacedCRD("Project", "projects"),
		minimalClusterCRD("Tenant", "tenants"),
		minimalClusterCRD("BillingUsage", "billingusages"),
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
		ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("%s.%s", strings.ToLower(plural), paasGroup)},
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
					Subresources: &apiextensionsv1.CustomResourceSubresources{Status: &apiextensionsv1.CustomResourceSubresourceStatus{}},
				},
			},
		},
	}
}

const paasGroup = "paas.kubeop.io"

func ptr[T any](v T) *T { return &v }
