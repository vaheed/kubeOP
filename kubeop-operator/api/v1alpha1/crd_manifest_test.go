package v1alpha1

import (
	"os"
	"path/filepath"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

func TestAppCRDManifestAlignment(t *testing.T) {
	t.Parallel()

	manifestPath := filepath.Join("..", "..", "config", "crd", "bases", "kubeop.io_apps.yaml")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("read CRD manifest: %v", err)
	}

	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(data, &crd); err != nil {
		t.Fatalf("unmarshal CRD manifest: %v", err)
	}

	if crd.Spec.Group != GroupVersion.Group {
		t.Fatalf("unexpected group: want %q, got %q", GroupVersion.Group, crd.Spec.Group)
	}

	if len(crd.Spec.Versions) == 0 {
		t.Fatal("expected at least one version in CRD manifest")
	}

	v := crd.Spec.Versions[0]
	if v.Name != GroupVersion.Version {
		t.Fatalf("unexpected version: want %q, got %q", GroupVersion.Version, v.Name)
	}

	if !v.Served {
		t.Error("expected version to be served")
	}

	if !v.Storage {
		t.Error("expected version to be marked as storage")
	}

	if crd.Spec.Scope != apiextensionsv1.NamespaceScoped {
		t.Fatalf("unexpected scope: want %s, got %s", apiextensionsv1.NamespaceScoped, crd.Spec.Scope)
	}

	if crd.Spec.Names.Kind != "App" {
		t.Fatalf("unexpected kind: want %q, got %q", "App", crd.Spec.Names.Kind)
	}
}
