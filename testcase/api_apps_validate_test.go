package testcase

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"kubeop/internal/api"
	"kubeop/internal/config"
	"kubeop/internal/service"
	"kubeop/internal/store"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestValidateAppHandler(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{
		DisableAuth:                true,
		KcfgEncryptionKey:          "unit-test",
		MaxLoadBalancersPerProject: 2,
		PaaSWildcardEnabled:        true,
		PaaSDomain:                 "example.test",
	}
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
	if err := netv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add net scheme: %v", err)
	}
	svc.SetKubeManager(fakeKM{client: fake.NewClientBuilder().WithScheme(scheme).Build()})

	now := time.Now()
	mock.ExpectQuery(`SELECT id, user_id, cluster_id, name, namespace, suspended, created_at, quota_overrides, kubeconfig_enc FROM projects WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs("proj-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "cluster_id", "name", "namespace", "suspended", "created_at", "quota_overrides", "kubeconfig_enc"}).
			AddRow("proj-1", "user-1", "cluster-1", "Project One", "tenant-ns", false, now, []byte("{}"), []byte("enc")))
	mock.ExpectQuery(`SELECT c\.id, c\.name`).
		WithArgs("cluster-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "owner", "contact", "environment", "region", "api_server", "description", "tags",
			"created_at", "last_seen", "status_id", "healthy", "message", "apiserver_version", "node_count",
			"checked_at", "details",
		}).AddRow(
			"cluster-1", "stage", nil, nil, nil, nil, nil, nil, []byte("[]"), now, nil, nil, nil, nil, nil, nil, nil, []byte("{}"),
		))

	body, err := json.Marshal(map[string]any{
		"projectId": "proj-1",
		"name":      "web",
		"image":     "nginx:1",
		"ports": []map[string]any{{
			"containerPort": 80,
			"servicePort":   80,
			"serviceType":   "ClusterIP",
		}},
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	r := api.NewRouter(cfg, svc)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/apps/validate", bytes.NewReader(body))
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["projectId"].(string) != "proj-1" {
		t.Fatalf("unexpected projectId: %#v", resp["projectId"])
	}
	if resp["source"].(string) != "image" {
		t.Fatalf("expected source image, got %#v", resp["source"])
	}
	if _, ok := resp["kubeName"].(string); !ok {
		t.Fatalf("expected kubeName in response")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestValidateAppHandler_GitManifests(t *testing.T) {
	repoDir := writeGitRepo(t, map[string]string{
		"configmap.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: sample\ndata:\n  key: value\n",
	})

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{
		DisableAuth:                true,
		KcfgEncryptionKey:          "unit-test",
		MaxLoadBalancersPerProject: 1,
		AllowGitFileProtocol:       true,
	}
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
	if err := netv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add net scheme: %v", err)
	}
	svc.SetKubeManager(fakeKM{client: fake.NewClientBuilder().WithScheme(scheme).Build()})

	now := time.Now()
	mock.ExpectQuery(`SELECT id, user_id, cluster_id, name, namespace, suspended, created_at, quota_overrides, kubeconfig_enc FROM projects WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs("proj-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "cluster_id", "name", "namespace", "suspended", "created_at", "quota_overrides", "kubeconfig_enc"}).
			AddRow("proj-1", "user-1", "cluster-1", "Project One", "tenant-ns", false, now, []byte("{}"), []byte("enc")))
	mock.ExpectQuery(`SELECT c\.id, c\.name`).
		WithArgs("cluster-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "owner", "contact", "environment", "region", "api_server", "description", "tags",
			"created_at", "last_seen", "status_id", "healthy", "message", "apiserver_version", "node_count",
			"checked_at", "details",
		}).AddRow(
			"cluster-1", "stage", nil, nil, nil, nil, nil, nil, []byte("[]"), now, nil, nil, nil, nil, nil, nil, nil, []byte("{}"),
		))

	body, err := json.Marshal(map[string]any{
		"projectId": "proj-1",
		"name":      "git-app",
		"git": map[string]any{
			"url": "file://" + repoDir,
			"ref": "refs/heads/master",
		},
	})
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}

	r := api.NewRouter(cfg, svc)
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/apps/validate", bytes.NewReader(body))
	r.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", rr.Code, rr.Body.String())
	}

	var resp map[string]any
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp["source"].(string) != "git:manifests" {
		t.Fatalf("expected git:manifests source, got %#v", resp["source"])
	}
	repo, ok := resp["gitRepo"].(string)
	if !ok || repo == "" {
		t.Fatalf("expected gitRepo in response, got %#v", resp["gitRepo"])
	}
	commit, ok := resp["gitCommit"].(string)
	if !ok || commit == "" {
		t.Fatalf("expected gitCommit in response, got %#v", resp["gitCommit"])
	}
	objs, ok := resp["renderedObjects"].([]any)
	if !ok || len(objs) == 0 {
		t.Fatalf("expected renderedObjects, got %#v", resp["renderedObjects"])
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
