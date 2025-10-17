package testcase

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	crclient "sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"kubeop/internal/config"
	"kubeop/internal/service"
	"kubeop/internal/sink"
	"kubeop/internal/store"
)

func newTestService(t *testing.T) (*service.Service, sqlmock.Sqlmock, func()) {
	t.Helper()
	cfg := &config.Config{
		AdminJWTSecret:          "secret",
		KcfgEncryptionKey:       strings.Repeat("a", 32),
		EventsDBEnabled:         true,
		ProjectsInUserNamespace: true,
		PodSecurityLevel:        "restricted",
	}
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	cleanup := func() { db.Close() }
	st := store.NewWithDB(db)
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	svc.SetLogger(zap.NewNop())
	return svc, mock, cleanup
}

func TestProcessWatcherEvents_InsertsProjectEvent(t *testing.T) {
	svc, mock, cleanup := newTestService(t)
	defer cleanup()

	now := time.Now()
	mock.ExpectQuery(`INSERT INTO project_events`).
		WithArgs(sqlmock.AnyArg(), "proj-1", sqlmock.AnyArg(), sqlmock.AnyArg(), "K8S_POD_ADDED", "INFO", "Pod web-0 added", sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(now))

	event := sink.Event{
		ClusterID: "cluster-1",
		EventType: "Added",
		Kind:      "Pod",
		Namespace: "user-1",
		Name:      "web-0",
		Labels: map[string]string{
			"kubeop.project-id": "proj-1",
			"kubeop.app-id":     "app-99",
		},
		Summary:  "Pod web-0 added",
		DedupKey: "abc#123",
	}

	res, err := svc.ProcessWatcherEvents(context.Background(), "cluster-1", []sink.Event{event})
	if err != nil {
		t.Fatalf("ProcessWatcherEvents: %v", err)
	}
	if res.Accepted != 1 || res.Dropped != 0 || res.Total != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestProcessWatcherEvents_DropsMissingProject(t *testing.T) {
	svc, mock, cleanup := newTestService(t)
	defer cleanup()

	event := sink.Event{ClusterID: "cluster-1", EventType: "Added", Kind: "Service", Name: "api"}
	res, err := svc.ProcessWatcherEvents(context.Background(), "cluster-1", []sink.Event{event})
	if err != nil {
		t.Fatalf("ProcessWatcherEvents: %v", err)
	}
	if res.Accepted != 0 || res.Dropped != 1 || res.Total != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestProcessWatcherEvents_ClusterMismatch(t *testing.T) {
	svc, mock, cleanup := newTestService(t)
	defer cleanup()

	event := sink.Event{ClusterID: "cluster-x", EventType: "Deleted", Kind: "Deployment", Labels: map[string]string{"kubeop.project-id": "proj-2"}}
	res, err := svc.ProcessWatcherEvents(context.Background(), "cluster-1", []sink.Event{event})
	if err != nil {
		t.Fatalf("ProcessWatcherEvents: %v", err)
	}
	if res.Accepted != 0 || res.Dropped != 1 || res.Total != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestProcessWatcherEvents_ImportsKubectlDeployment(t *testing.T) {
	svc, mock, cleanup := newTestService(t)
	defer cleanup()
	svc.SetLogger(zap.NewNop())

	now := time.Now()
	projectRows := sqlmock.NewRows([]string{"id", "user_id", "cluster_id", "name", "namespace", "suspended", "created_at"}).
		AddRow("proj-1", "user-1", "cluster-1", "kubectl", "user-1", false, now)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, user_id, cluster_id, name, namespace, COALESCE(suspended,false), created_at FROM projects WHERE cluster_id=$1 AND namespace=$2 AND deleted_at IS NULL ORDER BY created_at LIMIT 1`)).
		WithArgs("cluster-1", "user-1").
		WillReturnRows(projectRows)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, project_id, name, status, repo, webhook_secret, external_ref, source FROM apps WHERE external_ref = $1 AND deleted_at IS NULL`)).
		WithArgs("kubectl:cluster-1:user-1:deployment/web-03").
		WillReturnError(sql.ErrNoRows)

	mock.ExpectExec(`INSERT INTO apps`).
		WithArgs(sqlmock.AnyArg(), "proj-1", "web-03", "imported", sqlmock.AnyArg(), sqlmock.AnyArg(), "kubectl:cluster-1:user-1:deployment/web-03", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	mock.ExpectQuery(`INSERT INTO project_events`).
		WithArgs(sqlmock.AnyArg(), "proj-1", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(now))

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: "user-1", Name: "web-03"},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web-03"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "web-03"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "nginx:1.28"}}},
			},
		},
	}
	client := fakeclient.NewClientBuilder().WithScheme(scheme).WithObjects(dep).Build()
	svc.SetKubeManager(&fakeManager{client: client})

	event := sink.Event{
		ClusterID: "cluster-1",
		EventType: "Added",
		Kind:      "Deployment",
		Namespace: "user-1",
		Name:      "web-03",
		Summary:   "deployment created",
		DedupKey:  "dep#1",
	}

	res, err := svc.ProcessWatcherEvents(context.Background(), "cluster-1", []sink.Event{event})
	if err != nil {
		t.Fatalf("ProcessWatcherEvents: %v", err)
	}
	if res.Accepted != 1 || res.Dropped != 0 || res.Total != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

type fakeManager struct {
	client crclient.Client
}

func (f *fakeManager) GetOrCreate(ctx context.Context, id string, loader func(context.Context) ([]byte, error)) (crclient.Client, error) {
	return f.client, nil
}

func (f *fakeManager) GetClientset(ctx context.Context, id string, loader func(context.Context) ([]byte, error)) (kubernetes.Interface, error) {
	return nil, errors.New("not implemented")
}
