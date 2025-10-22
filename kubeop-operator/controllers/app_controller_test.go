package controllers

import (
	"context"
	"testing"

	appv1alpha1 "github.com/vaheed/kubeOP/kubeop-operator/api/v1alpha1"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	app.SetGeneration(1)
	app.SetResourceVersion("1")

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&appv1alpha1.App{}).
		WithObjects(app).
		Build()

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

	var reconciled appv1alpha1.App
	if err := fakeClient.Get(context.Background(), req.NamespacedName, &reconciled); err != nil {
		t.Fatalf("unable to fetch reconciled App: %v", err)
	}

	if got, want := reconciled.Status.ObservedGeneration, int64(1); got != want {
		t.Fatalf("ObservedGeneration mismatch: got %d, want %d", got, want)
	}

	cond := apimeta.FindStatusCondition(reconciled.Status.Conditions, appv1alpha1.AppConditionReady)
	if cond == nil {
		t.Fatal("expected Ready condition to be set")
	}
	if cond.Status != metav1.ConditionTrue {
		t.Fatalf("Ready condition status mismatch: got %s, want %s", cond.Status, metav1.ConditionTrue)
	}
	if cond.Reason != appv1alpha1.AppReadyReasonReconciled {
		t.Fatalf("Ready condition reason mismatch: got %s, want %s", cond.Reason, appv1alpha1.AppReadyReasonReconciled)
	}
	if cond.ObservedGeneration != 1 {
		t.Fatalf("Ready condition observed generation mismatch: got %d, want %d", cond.ObservedGeneration, 1)
	}

	// Run a second reconcile to confirm status updates remain idempotent.
	if _, err := reconciler.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("second reconcile returned error: %v", err)
	}

	var reconciledAgain appv1alpha1.App
	if err := fakeClient.Get(context.Background(), req.NamespacedName, &reconciledAgain); err != nil {
		t.Fatalf("unable to fetch App after second reconcile: %v", err)
	}
	if len(reconciledAgain.Status.Conditions) != len(reconciled.Status.Conditions) {
		t.Fatalf("expected conditions count to remain stable, got %d vs %d", len(reconciledAgain.Status.Conditions), len(reconciled.Status.Conditions))
	}
}

func TestAppReconciler_ReconcileMissing(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("AddToScheme error: %v", err)
	}

	logger := zap.NewNop().Sugar()
	reconciler := &AppReconciler{
		Client: fake.NewClientBuilder().
			WithScheme(scheme).
			WithStatusSubresource(&appv1alpha1.App{}).
			Build(),
		Scheme: scheme,
		Logger: logger,
	}

	req := ctrl.Request{NamespacedName: types.NamespacedName{Name: "missing", Namespace: "default"}}
	if _, err := reconciler.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("expected no error when App is missing, got %v", err)
	}
}
