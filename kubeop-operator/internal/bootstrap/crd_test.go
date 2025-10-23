package bootstrap

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap/zaptest"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func stubWait(fn func(context.Context, apiextensionsclient.Interface, string) error) func() {
	original := waitForCRDReadyFn
	waitForCRDReadyFn = fn
	return func() {
		waitForCRDReadyFn = original
	}
}

func TestEnsureAppCRDInstallsWhenMissing(t *testing.T) {
	t.Cleanup(stubWait(func(ctx context.Context, client apiextensionsclient.Interface, name string) error {
		return nil
	}))

	client := fake.NewSimpleClientset()
	logger := zaptest.NewLogger(t).Sugar()

	if err := ensureAppCRDWithClient(context.Background(), client, logger); err != nil {
		t.Fatalf("EnsureAppCRD returned error: %v", err)
	}

	crd, err := client.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), "apps.kubeop.io", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected App CRD to be created: %v", err)
	}

	if crd.Labels["app.kubernetes.io/name"] != "kubeop-operator" {
		t.Fatalf("expected CRD label to be preserved: got %q", crd.Labels["app.kubernetes.io/name"])
	}

	if crd.Spec.Group != "kubeop.io" {
		t.Fatalf("expected CRD group kubeop.io, got %q", crd.Spec.Group)
	}
}

func TestEnsureAppCRDNoUpdateWhenUnchanged(t *testing.T) {
	t.Cleanup(stubWait(func(ctx context.Context, client apiextensionsclient.Interface, name string) error {
		return nil
	}))

	manifest, err := loadAppCRD()
	if err != nil {
		t.Fatalf("loadAppCRD returned error: %v", err)
	}
	existing := manifest.DeepCopy()
	existing.ResourceVersion = "1"
	if existing.Labels == nil {
		existing.Labels = map[string]string{}
	}
	existing.Labels["custom"] = "preserve"
	existing.Status.Conditions = []apiextensionsv1.CustomResourceDefinitionCondition{{
		Type:   apiextensionsv1.Established,
		Status: apiextensionsv1.ConditionTrue,
	}}

	client := fake.NewSimpleClientset(existing)
	logger := zaptest.NewLogger(t).Sugar()

	if err := ensureAppCRDWithClient(context.Background(), client, logger); err != nil {
		t.Fatalf("EnsureAppCRD returned error: %v", err)
	}

	actions := client.Actions()
	for _, action := range actions {
		if action.Matches("update", "customresourcedefinitions") {
			t.Fatalf("expected no update call, got actions: %+v", actions)
		}
	}

	crd, err := client.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), existing.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get CRD: %v", err)
	}
	if crd.Labels["custom"] != "preserve" {
		t.Fatalf("expected custom label to be preserved, got %q", crd.Labels["custom"])
	}
}

func TestEnsureAppCRDUpdatesWhenManifestDiffers(t *testing.T) {
	t.Cleanup(stubWait(func(ctx context.Context, client apiextensionsclient.Interface, name string) error {
		return nil
	}))

	manifest, err := loadAppCRD()
	if err != nil {
		t.Fatalf("loadAppCRD returned error: %v", err)
	}
	existing := manifest.DeepCopy()
	existing.ResourceVersion = "1"
	specProperty := existing.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"]
	specProperty.Description = "outdated"
	existing.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"] = specProperty
	delete(existing.Labels, "app.kubernetes.io/component")

	client := fake.NewSimpleClientset(existing)
	logger := zaptest.NewLogger(t).Sugar()

	if err := ensureAppCRDWithClient(context.Background(), client, logger); err != nil {
		t.Fatalf("EnsureAppCRD returned error: %v", err)
	}

	var updated bool
	for _, action := range client.Actions() {
		if action.Matches("update", "customresourcedefinitions") {
			updated = true
			break
		}
	}
	if !updated {
		t.Fatalf("expected update action when manifest differs")
	}

	crd, err := client.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), existing.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get CRD: %v", err)
	}
	if crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["spec"].Description == "outdated" {
		t.Fatalf("expected CRD spec to be updated from manifest")
	}
	if crd.Labels["app.kubernetes.io/component"] != "crd" {
		t.Fatalf("expected manifest labels to be restored")
	}
}

func TestEmbeddedCRDMatchesConfigFile(t *testing.T) {
	t.Parallel()

	configPath := filepath.Join("..", "..", "config", "crd", "bases", "kubeop.io_apps.yaml")
	configBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config CRD: %v", err)
	}

	if string(configBytes) != string(appCRDManifest) {
		t.Fatalf("embedded CRD manifest differs from config/crd/bases copy")
	}
}
