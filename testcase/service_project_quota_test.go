package testcase

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"kubeop/internal/config"
	"kubeop/internal/service"
	"kubeop/internal/store"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type fakeQuotaKubeManager struct {
	client crclient.Client
}

func (f *fakeQuotaKubeManager) GetOrCreate(context.Context, string, func(context.Context) ([]byte, error)) (crclient.Client, error) {
	return f.client, nil
}

func (f *fakeQuotaKubeManager) GetClientset(context.Context, string, func(context.Context) ([]byte, error)) (kubernetes.Interface, error) {
	return nil, errors.New("not implemented")
}

func TestGetProjectQuota_ReturnsSnapshot(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{
		KcfgEncryptionKey:                           "unit-test",
		ProjectsInUserNamespace:                     false,
		NamespaceQuotaRequestsCPU:                   "2",
		NamespaceQuotaLimitsCPU:                     "4",
		NamespaceQuotaRequestsMemory:                "4Gi",
		NamespaceQuotaLimitsMemory:                  "8Gi",
		NamespaceQuotaRequestsEphemeral:             "10Gi",
		NamespaceQuotaLimitsEphemeral:               "20Gi",
		NamespaceQuotaPods:                          "30",
		NamespaceQuotaServices:                      "10",
		NamespaceQuotaServicesLoadBalancers:         "1",
		NamespaceQuotaConfigMaps:                    "100",
		NamespaceQuotaSecrets:                       "100",
		NamespaceQuotaPVCs:                          "10",
		NamespaceQuotaRequestsStorage:               "200Gi",
		NamespaceQuotaDeployments:                   "20",
		NamespaceQuotaReplicaSets:                   "40",
		NamespaceQuotaStatefulSets:                  "5",
		NamespaceQuotaJobs:                          "20",
		NamespaceQuotaCronJobs:                      "10",
		NamespaceQuotaIngresses:                     "10",
		NamespaceQuotaScopes:                        "NotBestEffort",
		NamespaceQuotaPriorityClasses:               "",
		NamespaceLRContainerMaxCPU:                  "2",
		NamespaceLRContainerMaxMemory:               "2Gi",
		NamespaceLRContainerMinCPU:                  "100m",
		NamespaceLRContainerMinMemory:               "128Mi",
		NamespaceLRContainerDefaultCPU:              "500m",
		NamespaceLRContainerDefaultMemory:           "512Mi",
		NamespaceLRContainerDefaultRequestCPU:       "300m",
		NamespaceLRContainerDefaultRequestMemory:    "256Mi",
		NamespaceLRContainerMaxEphemeral:            "2Gi",
		NamespaceLRContainerMinEphemeral:            "128Mi",
		NamespaceLRContainerDefaultEphemeral:        "512Mi",
		NamespaceLRContainerDefaultRequestEphemeral: "256Mi",
		NamespaceLRExtMax:                           "",
		NamespaceLRExtMin:                           "",
		NamespaceLRExtDefault:                       "",
		NamespaceLRExtDefaultRequest:                "",
		ProjectLRRequestCPU:                         "50m",
		ProjectLRRequestMemory:                      "64Mi",
		ProjectLRLimitCPU:                           "500m",
		ProjectLRLimitMemory:                        "512Mi",
		MaxLoadBalancersPerProject:                  1,
	}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	svc.SetLogger(newTestLogger())

	now := time.Now()
	quotaJSON := []byte(`{"pods":"10","services.loadbalancers":"2"}`)
	mock.ExpectQuery(`SELECT id, user_id, cluster_id, name, namespace, suspended, created_at, quota_overrides, kubeconfig_enc FROM projects WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs("proj-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "cluster_id", "name", "namespace", "suspended", "created_at", "quota_overrides", "kubeconfig_enc"}).
			AddRow("proj-1", "user-1", "cluster-1", "demo", "tenant-demo", false, now, quotaJSON, []byte("enc")))

	scheme := newScheme(t)
	rq := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{Name: "tenant-quota", Namespace: "tenant-demo"},
		Spec:       corev1.ResourceQuotaSpec{Hard: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("10")}},
		Status: corev1.ResourceQuotaStatus{
			Hard: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("10")},
			Used: corev1.ResourceList{corev1.ResourcePods: resource.MustParse("3")},
		},
	}
	svcLB := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "tenant-demo"},
		Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeLoadBalancer},
	}
	client := fake.NewClientBuilder().WithScheme(scheme).WithObjects(rq, svcLB).Build()
	svc.SetKubeManager(&fakeQuotaKubeManager{client: client})

	snapshot, err := svc.GetProjectQuota(context.Background(), "proj-1")
	if err != nil {
		t.Fatalf("GetProjectQuota: %v", err)
	}
	if snapshot.Project.ID != "proj-1" {
		t.Fatalf("expected project id proj-1, got %q", snapshot.Project.ID)
	}
	if snapshot.Defaults["pods"] != "30" {
		t.Fatalf("expected default pods 30, got %q", snapshot.Defaults["pods"])
	}
	if snapshot.Overrides["pods"] != "10" {
		t.Fatalf("expected pods override 10, got %#v", snapshot.Overrides)
	}
	if snapshot.Effective["pods"] != "10" {
		t.Fatalf("expected effective pods 10, got %q", snapshot.Effective["pods"])
	}
	if snapshot.ResourceQuota.Hard["pods"] != "10" {
		t.Fatalf("expected hard pods 10, got %q", snapshot.ResourceQuota.Hard["pods"])
	}
	if snapshot.ResourceQuota.Used["pods"] != "3" {
		t.Fatalf("expected used pods 3, got %q", snapshot.ResourceQuota.Used["pods"])
	}
	if snapshot.LoadBalancers.Default != 1 {
		t.Fatalf("expected default lb quota 1, got %d", snapshot.LoadBalancers.Default)
	}
	if snapshot.LoadBalancers.Override == nil || *snapshot.LoadBalancers.Override != 2 {
		t.Fatalf("expected load balancer override 2, got %#v", snapshot.LoadBalancers.Override)
	}
	if snapshot.LoadBalancers.Effective != 2 {
		t.Fatalf("expected effective lb quota 2, got %d", snapshot.LoadBalancers.Effective)
	}
	if snapshot.LoadBalancers.Used != 1 {
		t.Fatalf("expected used lb 1, got %d", snapshot.LoadBalancers.Used)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestGetProjectQuota_UserNamespaceMode(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test", ProjectsInUserNamespace: true}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	if _, err := svc.GetProjectQuota(context.Background(), "proj-2"); err == nil {
		t.Fatalf("expected error when projects share user namespace")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestGetProjectQuota_ProjectNotFound(t *testing.T) {
	t.Parallel()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test", ProjectsInUserNamespace: false}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}

	mock.ExpectQuery(`SELECT id, user_id, cluster_id, name, namespace, suspended, created_at, quota_overrides, kubeconfig_enc FROM projects WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)

	if _, err := svc.GetProjectQuota(context.Background(), "missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
