package bootstrap

import (
	"context"
	"testing"

	"go.uber.org/zap/zaptest"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func withWaitStub(t *testing.T, fn func(context.Context, apiextensionsclient.Interface, string) error) {
	t.Helper()
	original := waitForCRDReadyFn
	waitForCRDReadyFn = fn
	t.Cleanup(func() { waitForCRDReadyFn = original })
}

func TestEnsureCRDsInstallsMissing(t *testing.T) {
	withWaitStub(t, func(ctx context.Context, client apiextensionsclient.Interface, name string) error { return nil })

	client := fake.NewSimpleClientset()
	logger := zaptest.NewLogger(t).Sugar()

	if err := ensureCRDsWithClient(context.Background(), client, logger); err != nil {
		t.Fatalf("EnsureCRDs returned error: %v", err)
	}

	manifests, err := loadBundledCRDs()
	if err != nil {
		t.Fatalf("loadBundledCRDs returned error: %v", err)
	}

	for _, crd := range manifests {
		if _, err := client.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), crd.Name, metav1GetOptions); err != nil {
			t.Fatalf("expected CRD %s to be created: %v", crd.Name, err)
		}
	}
}

var metav1GetOptions = metav1.GetOptions{}

func TestEnsureCRDsUpdatesWhenSpecDiffers(t *testing.T) {
	withWaitStub(t, func(ctx context.Context, client apiextensionsclient.Interface, name string) error { return nil })

	manifests, err := loadBundledCRDs()
	if err != nil {
		t.Fatalf("loadBundledCRDs returned error: %v", err)
	}

	if len(manifests) == 0 {
		t.Fatalf("expected bundled CRDs")
	}
	var original *apiextensionsv1.CustomResourceDefinition
	for _, crd := range manifests {
		if len(crd.Spec.Versions) == 0 {
			continue
		}
		if crd.Spec.Versions[0].Schema == nil || crd.Spec.Versions[0].Schema.OpenAPIV3Schema == nil {
			continue
		}
		original = crd.DeepCopy()
		break
	}
	if original == nil {
		t.Skip("no CRDs with OpenAPI schemas available to test update path")
	}
	existing := original.DeepCopy()
	existing.ResourceVersion = "1"
	existing.Spec.Versions[0].Schema.OpenAPIV3Schema.Description = "outdated"

	client := fake.NewSimpleClientset(existing)
	logger := zaptest.NewLogger(t).Sugar()

	if err := ensureCRDsWithClient(context.Background(), client, logger); err != nil {
		t.Fatalf("EnsureCRDs returned error: %v", err)
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

	got, err := client.ApiextensionsV1().CustomResourceDefinitions().Get(context.Background(), original.Name, metav1GetOptions)
	if err != nil {
		t.Fatalf("get CRD: %v", err)
	}
	if got.Spec.Versions[0].Schema.OpenAPIV3Schema.Description == "outdated" {
		t.Fatalf("expected CRD schema to be refreshed")
	}
}
