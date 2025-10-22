package testcase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/kubernetes"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	"kubeop/internal/config"
	"kubeop/internal/service"
	"kubeop/internal/store"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

type fakeKMCluster struct {
	client    crclient.Client
	clientset kubernetes.Interface
	err       error
}

func (f fakeKMCluster) GetOrCreate(ctx context.Context, id string, loader func(context.Context) ([]byte, error)) (crclient.Client, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.client, nil
}

func (f fakeKMCluster) GetClientset(ctx context.Context, id string, loader func(context.Context) ([]byte, error)) (kubernetes.Interface, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.clientset, nil
}

func TestServiceRegisterClusterAppliesDefaults(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	cfg := &config.Config{KcfgEncryptionKey: "unit-test", ClusterDefaultEnvironment: "staging", ClusterDefaultRegion: "eu-west"}
	st := store.NewWithDB(db)
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)
	svc.SetEnsureOperatorFunc(func(context.Context, string, string) error { return nil })

	mock.ExpectQuery(`INSERT INTO clusters`).
		WithArgs(
			sqlmock.AnyArg(),
			"prod-cluster",
			"sre",
			nil,
			"staging",
			"eu-west",
			nil,
			nil,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now().UTC()))

	_, err = svc.RegisterCluster(context.Background(), service.ClusterRegisterInput{
		Name:       "prod-cluster",
		Kubeconfig: "apiVersion: v1",
		Metadata: service.ClusterMetadataInput{
			Owner: "sre",
			Tags:  []string{"Prod", "primary", "prod"},
		},
	})
	if err != nil {
		t.Fatalf("RegisterCluster: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestServiceRegisterClusterInstallsOperator(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	cfg := &config.Config{
		KcfgEncryptionKey:       "unit-test",
		OperatorNamespace:       "kubeop-system",
		OperatorDeploymentName:  "kubeop-operator",
		OperatorServiceAccount:  "kubeop-operator",
		OperatorImage:           "ghcr.io/example/kubeop-operator:main",
		OperatorImagePullPolicy: string(corev1.PullAlways),
		OperatorLeaderElection:  true,
	}
	st := store.NewWithDB(db)
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apps scheme: %v", err)
	}
	if err := rbacv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add rbac scheme: %v", err)
	}
	if err := apiextensionsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apiextensions scheme: %v", err)
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	svc.SetKubeManager(fakeKMCluster{client: fakeClient})

	mock.ExpectQuery(`INSERT INTO clusters`).
		WithArgs(
			sqlmock.AnyArg(),
			"demo",
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now().UTC()))

	cluster, err := svc.RegisterCluster(context.Background(), service.ClusterRegisterInput{
		Name:       "demo",
		Kubeconfig: "apiVersion: v1",
	})
	if err != nil {
		t.Fatalf("RegisterCluster: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}

	ctx := context.Background()
	var ns corev1.Namespace
	if err := fakeClient.Get(ctx, crclient.ObjectKey{Name: cfg.OperatorNamespace}, &ns); err != nil {
		t.Fatalf("expected namespace ensured: %v", err)
	}
	if ns.Labels["kubeop.io/managed"] != "true" {
		t.Fatalf("expected managed label on namespace")
	}

	var sa corev1.ServiceAccount
	if err := fakeClient.Get(ctx, crclient.ObjectKey{Name: cfg.OperatorServiceAccount, Namespace: cfg.OperatorNamespace}, &sa); err != nil {
		t.Fatalf("expected service account: %v", err)
	}

	var cr rbacv1.ClusterRole
	if err := fakeClient.Get(ctx, crclient.ObjectKey{Name: cfg.OperatorDeploymentName}, &cr); err != nil {
		t.Fatalf("expected cluster role: %v", err)
	}
	foundDeploymentRule := false
	for _, rule := range cr.Rules {
		if containsString(rule.APIGroups, "apps") && containsString(rule.Resources, "deployments") {
			foundDeploymentRule = true
			break
		}
	}
	if !foundDeploymentRule {
		t.Fatalf("expected cluster role to manage deployments: %#v", cr.Rules)
	}

	var binding rbacv1.ClusterRoleBinding
	if err := fakeClient.Get(ctx, crclient.ObjectKey{Name: fmt.Sprintf("%s-binding", cfg.OperatorDeploymentName)}, &binding); err != nil {
		t.Fatalf("expected cluster role binding: %v", err)
	}
	if len(binding.Subjects) != 1 || binding.Subjects[0].Namespace != cfg.OperatorNamespace {
		t.Fatalf("expected binding to reference operator service account: %#v", binding.Subjects)
	}

	var dep appsv1.Deployment
	if err := fakeClient.Get(ctx, crclient.ObjectKey{Name: cfg.OperatorDeploymentName, Namespace: cfg.OperatorNamespace}, &dep); err != nil {
		t.Fatalf("expected deployment: %v", err)
	}
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 1 {
		t.Fatalf("expected single replica, got %#v", dep.Spec.Replicas)
	}
	if len(dep.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("expected single container")
	}
	container := dep.Spec.Template.Spec.Containers[0]
	if container.Image != cfg.OperatorImage {
		t.Fatalf("expected image %q, got %q", cfg.OperatorImage, container.Image)
	}
	if container.ImagePullPolicy != corev1.PullAlways {
		t.Fatalf("expected pull policy Always, got %s", container.ImagePullPolicy)
	}
	if !containsString(container.Args, "--leader-elect") {
		t.Fatalf("expected leader election arg")
	}

	var crd apiextensionsv1.CustomResourceDefinition
	if err := fakeClient.Get(ctx, crclient.ObjectKey{Name: "apps.app.kubeop.io"}, &crd); err != nil {
		t.Fatalf("expected crd: %v", err)
	}
	if crd.Spec.Group != "app.kubeop.io" || len(crd.Spec.Versions) == 0 {
		t.Fatalf("unexpected crd spec: %#v", crd.Spec)
	}
	if len(crd.Spec.Versions[0].AdditionalPrinterColumns) == 0 {
		t.Fatalf("expected printer columns")
	}

	if cluster.ID == "" {
		t.Fatalf("expected cluster id to be set")
	}
}

func TestServiceRegisterClusterOperatorFailureRollsBack(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	cfg := &config.Config{KcfgEncryptionKey: "unit-test"}
	st := store.NewWithDB(db)
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)

	expected := errors.New("boom")
	svc.SetEnsureOperatorFunc(func(context.Context, string, string) error { return expected })

	mock.ExpectQuery(`INSERT INTO clusters`).
		WithArgs(
			sqlmock.AnyArg(),
			"demo",
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(time.Now().UTC()))
	mock.ExpectExec(`DELETE FROM clusters`).
		WithArgs(sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	_, err = svc.RegisterCluster(context.Background(), service.ClusterRegisterInput{Name: "demo", Kubeconfig: "apiVersion: v1"})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "install kubeop-operator") {
		t.Fatalf("expected operator error, got %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestServiceCheckClusterSuccessPersistsStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	cfg := &config.Config{KcfgEncryptionKey: "unit-test"}
	st := store.NewWithDB(db)
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	client := fake.NewClientBuilder().WithScheme(scheme).Build()
	clientset := k8sfake.NewSimpleClientset()
	if disc, ok := clientset.Discovery().(*fakediscovery.FakeDiscovery); ok {
		disc.FakedServerVersion = &version.Info{GitVersion: "v1.30.0"}
	}
	svc.SetKubeManager(fakeKMCluster{client: client, clientset: clientset})

	now := time.Now().UTC()
	mock.ExpectQuery(`SELECT c.id, c.name`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "owner", "contact", "environment", "region", "api_server", "description", "tags", "created_at", "last_seen",
			"status_id", "healthy", "message", "apiserver_version", "node_count", "checked_at", "details",
		}).AddRow("cluster-1", "prod", nil, nil, nil, nil, nil, nil, []byte("[]"), now, nil, nil, nil, nil, nil, nil, nil, nil))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO cluster_status`).
		WithArgs(sqlmock.AnyArg(), "cluster-1", true, "connected", "v1.30.0", nil, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"checked_at"}).AddRow(time.Now().UTC()))
	mock.ExpectExec(`UPDATE clusters SET last_status_id =`).
		WithArgs("cluster-1", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	health, err := svc.CheckCluster(context.Background(), "cluster-1")
	if err != nil {
		t.Fatalf("CheckCluster: %v", err)
	}
	if !health.Healthy || health.Message != "connected" {
		t.Fatalf("expected healthy status, got %+v", health)
	}
	if health.APIServerVersion != "v1.30.0" {
		t.Fatalf("expected api server version to propagate, got %q", health.APIServerVersion)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestServiceCheckClusterFailurePersistsStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	cfg := &config.Config{KcfgEncryptionKey: "unit-test"}
	st := store.NewWithDB(db)
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)

	svc.SetKubeManager(fakeKMCluster{err: context.DeadlineExceeded})

	now := time.Now().UTC()
	mock.ExpectQuery(`SELECT c.id, c.name`).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "owner", "contact", "environment", "region", "api_server", "description", "tags", "created_at", "last_seen",
			"status_id", "healthy", "message", "apiserver_version", "node_count", "checked_at", "details",
		}).AddRow("cluster-1", "prod", nil, nil, nil, nil, nil, nil, []byte("[]"), now, nil, nil, nil, nil, nil, nil, nil, nil))

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO cluster_status`).
		WithArgs(sqlmock.AnyArg(), "cluster-1", false, sqlmock.AnyArg(), nil, nil, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"checked_at"}).AddRow(time.Now().UTC()))
	mock.ExpectExec(`UPDATE clusters SET last_status_id =`).
		WithArgs("cluster-1", sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	health, err := svc.CheckCluster(context.Background(), "cluster-1")
	if err != nil {
		t.Fatalf("CheckCluster: %v", err)
	}
	if health.Healthy {
		t.Fatalf("expected unhealthy status, got %+v", health)
	}
	if health.Error == "" {
		t.Fatalf("expected error message to surface")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func containsString(values []string, target string) bool {
	for _, v := range values {
		if v == target {
			return true
		}
	}
	return false
}
