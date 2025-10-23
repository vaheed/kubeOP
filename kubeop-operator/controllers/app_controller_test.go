package controllers

import (
	"context"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

        appv1alpha1 "github.com/vaheed/kubeOP/kubeop-operator/apis/paas/v1alpha1"
	"go.uber.org/zap"
)

func newTestReconciler(t *testing.T, objects ...ctrlclient.Object) *AppReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apps scheme: %v", err)
	}
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add app scheme: %v", err)
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
	logger := zap.NewNop().Sugar()
	return &AppReconciler{Client: client, Scheme: scheme, Logger: logger}
}

func TestReconcileWorkloadCreatesDeployment(t *testing.T) {
        app := &appv1alpha1.App{
                ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "default"},
                Spec: appv1alpha1.AppSpec{
                        Type: appv1alpha1.AppTypeRaw,
                        Source: &appv1alpha1.AppSource{
                                URL: "ghcr.io/example/demo",
                                Ref: "1.21",
                        },
                },
        }
        r := newTestReconciler(t)
        if err := r.reconcileWorkload(context.Background(), r.Logger, app); err != nil {
                t.Fatalf("reconcileWorkload: %v", err)
        }
        var dep appsv1.Deployment
        if err := r.Get(context.Background(), types.NamespacedName{Name: "demo", Namespace: "default"}, &dep); err != nil {
                t.Fatalf("get deployment: %v", err)
        }
        if dep.Spec.Template.Spec.Containers[0].Image != "ghcr.io/example/demo:1.21" {
                t.Fatalf("expected image ghcr.io/example/demo:1.21, got %s", dep.Spec.Template.Spec.Containers[0].Image)
        }
}

func TestReconcileWorkloadPrunesOldDeployment(t *testing.T) {
	existing := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stale",
			Namespace: "default",
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "kubeop-operator",
			},
		},
	}
        app := &appv1alpha1.App{
                ObjectMeta: metav1.ObjectMeta{Name: "fresh", Namespace: "default"},
                Spec: appv1alpha1.AppSpec{
                        Type: appv1alpha1.AppTypeRaw,
                        Source: &appv1alpha1.AppSource{
                                URL: "ghcr.io/example/fresh",
                        },
                },
        }
	r := newTestReconciler(t, existing)
	if err := r.reconcileWorkload(context.Background(), r.Logger, app); err != nil {
		t.Fatalf("reconcileWorkload: %v", err)
	}
	if err := r.Get(context.Background(), types.NamespacedName{Name: "stale", Namespace: "default"}, &appsv1.Deployment{}); err == nil {
		t.Fatalf("expected stale deployment to be pruned")
	}
}
