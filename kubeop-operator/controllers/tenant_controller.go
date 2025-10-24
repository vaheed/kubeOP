package controllers

import (
	"context"
	"fmt"

	appv1alpha1 "github.com/vaheed/kubeOP/kubeop-operator/apis/paas/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	labelTenant                = "paas.kubeop.io/tenant"
	tenantOwnerClusterRole     = "tenant-owner"
	tenantDeveloperClusterRole = "tenant-developer"
	tenantViewerClusterRole    = "tenant-viewer"
	managedRoleBindingLabel    = "rbac.kubeop.io/managed"
)

// TenantReconciler manages tenant-scoped RBAC templates.
type TenantReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Logger *zap.SugaredLogger
}

// SetupWithManager wires the reconciler into the controller manager.
func (r *TenantReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1alpha1.Tenant{}).
		WatchesRawSource(source.Kind(mgr.GetCache(), &corev1.Namespace{}), handler.EnqueueRequestsFromMapFunc(r.enqueueNamespace)).
		Complete(r)
}

// Reconcile ensures tenant role bindings exist across namespaces.
func (r *TenantReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := r.Logger.With("tenant", req.NamespacedName.Name)

	var tenant appv1alpha1.Tenant
	if err := r.Get(ctx, req.NamespacedName, &tenant); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("fetch tenant: %w", err)
	}

	namespaces, err := r.namespacesForTenant(ctx, tenant.Name)
	if err != nil {
		return ctrl.Result{}, err
	}

	for _, ns := range namespaces {
		if err := r.ensureRoleBinding(ctx, &tenant, &ns, tenantOwnerClusterRole, fmt.Sprintf("tenant:%s:owners", tenant.Name)); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.ensureRoleBinding(ctx, &tenant, &ns, tenantDeveloperClusterRole, fmt.Sprintf("tenant:%s:developers", tenant.Name)); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.ensureRoleBinding(ctx, &tenant, &ns, tenantViewerClusterRole, fmt.Sprintf("tenant:%s:viewers", tenant.Name)); err != nil {
			return ctrl.Result{}, err
		}
	}

	logger.Infow("synced tenant role bindings", "namespaces", len(namespaces))
	return ctrl.Result{}, nil
}

func (r *TenantReconciler) namespacesForTenant(ctx context.Context, tenant string) ([]corev1.Namespace, error) {
	var list corev1.NamespaceList
	if err := r.List(ctx, &list, client.MatchingLabels{labelTenant: tenant}); err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}
	return list.Items, nil
}

func (r *TenantReconciler) ensureRoleBinding(ctx context.Context, tenant *appv1alpha1.Tenant, ns *corev1.Namespace, roleName, group string) error {
	bindingName := fmt.Sprintf("kubeop:%s", roleName)
	rb := &rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{Name: bindingName, Namespace: ns.Name}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, rb, func() error {
		if rb.Labels == nil {
			rb.Labels = map[string]string{}
		}
		rb.Labels[labelTenant] = tenant.Name
		rb.Labels[managedRoleBindingLabel] = "true"
		rb.RoleRef = rbacv1.RoleRef{APIGroup: "rbac.authorization.k8s.io", Kind: "ClusterRole", Name: roleName}
		rb.Subjects = []rbacv1.Subject{{Kind: rbacv1.GroupKind, Name: group}}
		return nil
	})
	if err != nil {
		return fmt.Errorf("sync rolebinding %s/%s: %w", ns.Name, bindingName, err)
	}
	return nil
}

func (r *TenantReconciler) enqueueNamespace(ctx context.Context, obj client.Object) []reconcile.Request {
	tenant := obj.GetLabels()[labelTenant]
	if tenant == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: types.NamespacedName{Name: tenant}}}
}
