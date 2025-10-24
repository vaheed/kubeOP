package testcase

import (
	"context"
	"testing"

	appv1alpha1 "github.com/vaheed/kubeOP/kubeop-operator/apis/paas/v1alpha1"
	"github.com/vaheed/kubeOP/kubeop-operator/controllers"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestServiceTenantRBACBindings(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add client-go scheme: %v", err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add app scheme: %v", err)
	}
	if err := rbacv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add rbac scheme: %v", err)
	}

	tenant := &appv1alpha1.Tenant{ObjectMeta: metav1.ObjectMeta{Name: "acme"}}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "team-a", Labels: map[string]string{"paas.kubeop.io/tenant": "acme"}}}
	client := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(tenant, ns).Build()
	reconciler := &controllers.TenantReconciler{Client: client, Scheme: scheme, Logger: zap.NewNop().Sugar()}

	if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: "acme"}}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	for _, role := range []string{"tenant-owner", "tenant-developer", "tenant-viewer"} {
		rb := &rbacv1.RoleBinding{}
		if err := client.Get(context.Background(), types.NamespacedName{Name: "kubeop:" + role, Namespace: ns.Name}, rb); err != nil {
			t.Fatalf("expected rolebinding %s: %v", role, err)
		}
		if rb.Labels["paas.kubeop.io/tenant"] != "acme" {
			t.Fatalf("rolebinding %s missing tenant label", role)
		}
	}
}
