package testcase

import (
	"context"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"kubeop/internal/config"
	"kubeop/internal/service"
	"kubeop/internal/store"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type fakeKM struct{ client client.Client }

func (f fakeKM) GetOrCreate(ctx context.Context, id string, loader func(context.Context) ([]byte, error)) (client.Client, error) {
	return f.client, nil
}

func (f fakeKM) GetClientset(ctx context.Context, id string, loader func(context.Context) ([]byte, error)) (kubernetes.Interface, error) {
	return nil, errors.New("not implemented")
}

func TestServiceValidateApp_ImageSource(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{
		KcfgEncryptionKey:          "unit-test",
		MaxLoadBalancersPerProject: 2,
		PaaSWildcardEnabled:        true,
		PaaSDomain:                 "example.test",
	}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apps scheme: %v", err)
	}
	if err := netv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add net scheme: %v", err)
	}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	svc.SetKubeManager(fakeKM{client: fakeClient})

	now := time.Now()
	mock.ExpectQuery(`SELECT id, user_id, cluster_id, name, namespace, suspended, created_at, quota_overrides, kubeconfig_enc FROM projects WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs("proj-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "cluster_id", "name", "namespace", "suspended", "created_at", "quota_overrides", "kubeconfig_enc"}).
			AddRow("proj-1", "user-1", "cluster-1", "Project One", "tenant-ns", false, now, []byte("{}"), []byte("enc")))
	mock.ExpectQuery(`SELECT id, name, created_at FROM clusters WHERE id = \$1`).
		WithArgs("cluster-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "created_at"}).AddRow("cluster-1", "stage", now))

	out, err := svc.ValidateApp(context.Background(), service.AppDeployInput{
		ProjectID: "proj-1",
		Name:      "web",
		Image:     "nginx:1",
		Ports: []service.AppPort{{
			ContainerPort: 80,
			ServicePort:   80,
			ServiceType:   "LoadBalancer",
		}},
	})
	if err != nil {
		t.Fatalf("ValidateApp returned error: %v", err)
	}
	if out.ProjectID != "proj-1" {
		t.Fatalf("expected projectId proj-1, got %s", out.ProjectID)
	}
	if out.Source != "image" {
		t.Fatalf("expected source image, got %s", out.Source)
	}
	if out.LoadBalancers.Requested != 1 || out.LoadBalancers.Limit != 2 {
		t.Fatalf("unexpected lb summary: %#v", out.LoadBalancers)
	}
	if out.Domain == "" {
		t.Fatalf("expected generated domain, got empty")
	}
	if len(out.RenderedObjects) == 0 {
		t.Fatalf("expected rendered object summaries")
	}
	kinds := map[string]bool{}
	for _, ro := range out.RenderedObjects {
		kinds[ro.Kind] = true
	}
	if !kinds["Deployment"] || !kinds["Service"] {
		t.Fatalf("expected deployment and service summaries, got %#v", out.RenderedObjects)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
