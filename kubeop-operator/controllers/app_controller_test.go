package controllers

import (
	"context"
	"testing"

	appv1alpha1 "github.com/vaheed/kubeOP/kubeop-operator/api/v1alpha1"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestAppReconciler_Reconcile(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme error: %v", err)
	}

	app := &appv1alpha1.App{}
	app.SetName("example")
	app.SetNamespace("default")

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).Build()

	logger := zaptest.NewLogger(t)
	reconciler := &AppReconciler{
		Client: fakeClient,
		Scheme: scheme,
		Logger: logger.Sugar(),
	}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: app.GetName(), Namespace: app.GetNamespace()}}
	if _, err := reconciler.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("Reconcile returned error: %v", err)
	}
}

func TestAppReconciler_ReconcileMissing(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme error: %v", err)
	}

	logger := zap.NewNop().Sugar()
	reconciler := &AppReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme).Build(),
		Scheme: scheme,
		Logger: logger,
	}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "missing", Namespace: "default"}}
	if _, err := reconciler.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("expected no error when App is missing, got %v", err)
	}
}
