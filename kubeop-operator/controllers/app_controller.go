package controllers

import (
	"context"

	appv1alpha1 "github.com/vaheed/kubeOP/kubeop-operator/api/v1alpha1"
	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// AppReconciler reconciles App resources produced by kubeOP.
type AppReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Logger *zap.SugaredLogger
}

// Reconcile processes a single reconciliation request for an App resource.
func (r *AppReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Logger.With("controller", "App", "namespace", req.Namespace, "name", req.Name)
	log.Infow("Reconciling App")

	var app appv1alpha1.App
	if err := r.Get(ctx, req.NamespacedName, &app); err != nil {
		if apierrors.IsNotFound(err) {
			log.Infow("App resource no longer exists; nothing to do")
			return ctrl.Result{}, nil
		}
		log.Errorw("Unable to fetch App", "error", err)
		return ctrl.Result{}, err
	}

	log.Infow("App reconcile completed", "generation", app.GetGeneration())
	return ctrl.Result{}, nil
}

// SetupWithManager wires the AppReconciler into a controller-runtime manager.
func (r *AppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1alpha1.App{}).
		Complete(r)
}
