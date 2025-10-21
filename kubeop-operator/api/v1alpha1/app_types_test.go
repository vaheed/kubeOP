package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestAddToScheme(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme() error = %v", err)
	}

	gvk := GroupVersion.WithKind("App")
	if _, err := scheme.New(gvk); err != nil {
		t.Fatalf("expected App kind to be registered: %v", err)
	}
}

func TestAppDeepCopy(t *testing.T) {
	replicas := int32(3)
	original := &App{
		Spec: AppSpec{
			Image:    "ghcr.io/example/app:1.0.0",
			Replicas: &replicas,
			Hosts:    []string{"app.example.com"},
		},
		Status: AppStatus{
			ObservedGeneration: 5,
			AvailableReplicas:  2,
		},
	}

	clone := original.DeepCopy()
	if clone == original {
		t.Fatal("DeepCopy() should allocate a new App instance")
	}

	if clone.Spec.Replicas == original.Spec.Replicas {
		t.Fatal("DeepCopy() should copy replica pointer")
	}

	if got, want := clone.Spec.Image, original.Spec.Image; got != want {
		t.Fatalf("DeepCopy() image mismatch: got %s, want %s", got, want)
	}

	clone.Spec.Hosts[0] = "other.example.com"
	if original.Spec.Hosts[0] == clone.Spec.Hosts[0] {
		t.Fatal("DeepCopy() should isolate host slices")
	}
}
