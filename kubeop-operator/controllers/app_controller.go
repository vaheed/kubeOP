package controllers

import (
	"context"

	appv1alpha1 "github.com/vaheed/kubeOP/kubeop-operator/api/v1alpha1"
	"go.uber.org/zap"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	if err := r.updateStatus(ctx, log, &app); err != nil {
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

func (r *AppReconciler) updateStatus(ctx context.Context, log *zap.SugaredLogger, app *appv1alpha1.App) error {
	currentStatus := *app.Status.DeepCopy()

	readyCondition := metav1.Condition{
		Type:               appv1alpha1.AppConditionReady,
		Status:             metav1.ConditionTrue,
		Reason:             appv1alpha1.AppReadyReasonReconciled,
		Message:            "App reconciliation completed successfully",
		ObservedGeneration: app.GetGeneration(),
	}

	app.Status.ObservedGeneration = app.GetGeneration()
	apimeta.SetStatusCondition(&app.Status.Conditions, readyCondition)

	if apiequality.Semantic.DeepEqual(currentStatus, app.Status) {
		log.Infow("App status already up to date", "generation", app.GetGeneration())
		return nil
	}

	if err := r.Status().Update(ctx, app); err != nil {
		log.Errorw("Failed to update App status", "error", err)
		return err
	}

	log.Infow("Updated App status", "observedGeneration", app.Status.ObservedGeneration)
	return nil
}
