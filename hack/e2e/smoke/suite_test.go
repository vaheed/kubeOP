//go:build e2e

package smoke

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func kubeConfig(t *testing.T) *rest.Config {
	t.Helper()
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		t.Fatal("KUBECONFIG must be set for e2e tests")
	}
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		t.Fatalf("build config: %v", err)
	}
	config.Timeout = 30 * time.Second
	return config
}

func kubeClients(t *testing.T) (*kubernetes.Clientset, dynamic.Interface) {
	cfg := kubeConfig(t)
	k, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("kube client: %v", err)
	}
	d, err := dynamic.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("dynamic client: %v", err)
	}
	return k, d
}

func TestTenantsProjectsAppsFlow(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	_, dyn := kubeClients(t)

	tenantGVR := schema.GroupVersionResource{Group: "paas.kubeop.io", Version: "v1alpha1", Resource: "tenants"}
	projectGVR := schema.GroupVersionResource{Group: "paas.kubeop.io", Version: "v1alpha1", Resource: "projects"}
	appGVR := schema.GroupVersionResource{Group: "paas.kubeop.io", Version: "v1alpha1", Resource: "apps"}

	waitForResource(ctx, t, dyn, tenantGVR, "", "demo-tenant")
	waitForResource(ctx, t, dyn, projectGVR, "", "demo-project")

	app := waitForResource(ctx, t, dyn, appGVR, "kubeop", "demo-app")
	spec := app.Object["spec"].(map[string]any)
	if specType, ok := spec["type"].(string); !ok || specType != "Image" {
		t.Fatalf("unexpected app type: %+v", spec)
	}
	if host, ok := spec["host"].(string); !ok || host == "" {
		t.Fatalf("app host missing: %+v", spec)
	}
}

func TestRBACAndQuotas(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	clientset, _ := kubeClients(t)

	if _, err := clientset.RbacV1().ClusterRoles().Get(ctx, "kubeop-operator", metav1.GetOptions{}); err != nil {
		t.Fatalf("cluster role missing: %v", err)
	}
	if _, err := clientset.RbacV1().ClusterRoleBindings().Get(ctx, "kubeop-operator", metav1.GetOptions{}); err != nil {
		t.Fatalf("cluster role binding missing: %v", err)
	}

	quotas, err := clientset.CoreV1().ResourceQuotas("kubeop").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list quotas: %v", err)
	}
	if len(quotas.Items) == 0 {
		t.Fatalf("expected default quota in kubeop namespace")
	}
}

func TestNetworkPolicyAndIngress(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	clientset, _ := kubeClients(t)

	policies, err := clientset.NetworkingV1().NetworkPolicies("kubeop").List(ctx, metav1.ListOptions{})
	if err != nil {
		t.Fatalf("list network policies: %v", err)
	}
	if len(policies.Items) == 0 {
		t.Fatalf("expected at least one network policy")
	}
}

func TestMetricsAndBillingSignals(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	clientset, _ := kubeClients(t)

	svc, err := clientset.CoreV1().Services("kubeop-system").Get(ctx, "kubeop-operator", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("metrics service missing: %v", err)
	}
	if len(svc.Spec.Ports) == 0 {
		t.Fatalf("metrics service has no ports: %+v", svc.Spec)
	}

	cm, err := clientset.CoreV1().ConfigMaps("kubeop-system").Get(ctx, "kubeop-billing-mock", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("billing configmap missing: %v", err)
	}
	var usage struct {
		Tenants []map[string]any `json:"tenants"`
	}
	if err := json.Unmarshal([]byte(cm.Data["usage.json"]), &usage); err != nil {
		t.Fatalf("parse usage: %v", err)
	}
	if len(usage.Tenants) == 0 {
		t.Fatalf("usage payload empty: %+v", usage)
	}
}

func waitForResource(ctx context.Context, t *testing.T, dyn dynamic.Interface, gvr schema.GroupVersionResource, namespace, name string) *unstructured.Unstructured {
	t.Helper()
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("timeout waiting for %s/%s", gvr.Resource, name)
		case <-ticker.C:
			var res *unstructured.Unstructured
			var err error
			if namespace == "" {
				res, err = dyn.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
			} else {
				res, err = dyn.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
			}
			if err == nil {
				return res
			}
		}
	}
}
