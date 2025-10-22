package controllers

import (
	"context"
	"fmt"

	appv1alpha1 "github.com/vaheed/kubeOP/kubeop-operator/api/v1alpha1"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

	if err := r.reconcileWorkload(ctx, log, &app); err != nil {
		log.Errorw("Failed to reconcile workload", "error", err)
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

func (r *AppReconciler) reconcileWorkload(ctx context.Context, log *zap.SugaredLogger, app *appv1alpha1.App) error {
	desired := buildDesiredResources(app)
	for _, obj := range desired {
		if err := controllerutil.SetControllerReference(app, obj, r.Scheme); err != nil {
			return fmt.Errorf("set owner reference: %w", err)
		}
	}
	for _, obj := range desired {
		kind := obj.GetObjectKind().GroupVersionKind().Kind
		name := obj.GetName()
		log.Infow("delivery.validate", "kind", kind, "name", name)
		dryRun := obj.DeepCopyObject().(client.Object)
		if err := r.Patch(ctx, dryRun, client.Apply, client.FieldOwner("kubeop-operator"), client.DryRunAll, client.ForceOwnership); err != nil {
			return fmt.Errorf("dry-run apply %s/%s: %w", kind, name, err)
		}
		log.Infow("delivery.apply", "kind", kind, "name", name)
		if err := r.Patch(ctx, obj, client.Apply, client.FieldOwner("kubeop-operator"), client.ForceOwnership); err != nil {
			return fmt.Errorf("apply %s/%s: %w", kind, name, err)
		}
	}
	if err := r.pruneWorkload(ctx, log, app, desired); err != nil {
		return err
	}
	return nil
}

func buildDesiredResources(app *appv1alpha1.App) []client.Object {
	labels := map[string]string{
		"app.kubernetes.io/name":       app.Name,
		"app.kubernetes.io/managed-by": "kubeop-operator",
	}
	replicas := int32(1)
	if app.Spec.Replicas != nil {
		replicas = *app.Spec.Replicas
	}
	dep := &appsv1.Deployment{}
	dep.SetGroupVersionKind(appsv1.SchemeGroupVersion.WithKind("Deployment"))
	dep.Name = app.Name
	dep.Namespace = app.Namespace
	dep.Labels = labels
	dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: labels}
	dep.Spec.Replicas = &replicas
	dep.Spec.Template.Labels = labels
	dep.Spec.Template.Spec.Containers = []corev1.Container{{
		Name:  "app",
		Image: app.Spec.Image,
	}}
	return []client.Object{dep}
}

func (r *AppReconciler) pruneWorkload(ctx context.Context, log *zap.SugaredLogger, app *appv1alpha1.App, desired []client.Object) error {
	desiredKeys := make(map[types.NamespacedName]struct{}, len(desired))
	for _, obj := range desired {
		desiredKeys[types.NamespacedName{Namespace: obj.GetNamespace(), Name: obj.GetName()}] = struct{}{}
	}
	var deployments appsv1.DeploymentList
	if err := r.List(ctx, &deployments, client.InNamespace(app.Namespace), client.MatchingLabels{"app.kubernetes.io/managed-by": "kubeop-operator"}); err != nil {
		return fmt.Errorf("list managed deployments: %w", err)
	}
	for _, existing := range deployments.Items {
		key := types.NamespacedName{Namespace: existing.Namespace, Name: existing.Name}
		if _, keep := desiredKeys[key]; keep {
			continue
		}
		log.Infow("delivery.prune", "kind", "Deployment", "name", existing.Name)
		if err := r.Delete(ctx, &existing); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("prune deployment %s: %w", existing.Name, err)
		}
	}
	return nil
}
