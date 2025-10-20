package testcase

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"kubeop/internal/config"
	"kubeop/internal/dns"
	"kubeop/internal/service"
	"kubeop/internal/store"
)

func TestServiceListAppReleases_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}

	appSource, _ := json.Marshal(map[string]any{"image": "nginx"})
	mock.ExpectQuery(`SELECT id, project_id, name, status, repo, webhook_secret, external_ref, source FROM apps`).
		WithArgs("app-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "name", "status", "repo", "webhook_secret", "external_ref", "source"}).
			AddRow("app-1", "proj-1", "web", "deployed", nil, nil, nil, appSource))

	specJSON, _ := json.Marshal(map[string]any{"name": "web"})
	renderedJSON, _ := json.Marshal([]map[string]any{{"kind": "Deployment", "name": "web"}})
	lbJSON, _ := json.Marshal(map[string]any{"requested": 1, "existing": 0, "limit": 5})
	warnJSON, _ := json.Marshal([]string{})
	helmVals, _ := json.Marshal(map[string]any{})
	now := time.Now()

	rows := sqlmock.NewRows([]string{
		"id", "project_id", "app_id", "source", "spec_digest", "render_digest",
		"spec", "rendered_objects", "load_balancers", "warnings",
		"helm_chart", "helm_values", "helm_render_sha", "manifests_sha", "repo",
		"status", "message", "created_at",
	}).
		AddRow("rel-1", "proj-1", "app-1", "image", "spec1", "render1", specJSON, renderedJSON, lbJSON, warnJSON, nil, helmVals, nil, nil, nil, "succeeded", "", now).
		AddRow("rel-0", "proj-1", "app-1", "image", "spec0", "render0", specJSON, renderedJSON, lbJSON, warnJSON, nil, helmVals, nil, nil, nil, "succeeded", "", now.Add(-time.Minute))

	mock.ExpectQuery(`FROM releases WHERE project_id = \$1 AND app_id = \$2 ORDER BY created_at DESC, id DESC LIMIT \$3`).
		WithArgs("proj-1", "app-1", 3).
		WillReturnRows(rows)

	page, err := svc.ListAppReleases(context.Background(), "proj-1", "app-1", 2, "")
	if err != nil {
		t.Fatalf("ListAppReleases: %v", err)
	}
	if len(page.Releases) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(page.Releases))
	}
	if page.Releases[0].ID != "rel-1" {
		t.Fatalf("unexpected first release: %#v", page.Releases[0])
	}
	if page.Releases[0].Spec.Name != "web" {
		t.Fatalf("spec not decoded: %#v", page.Releases[0].Spec)
	}
	if page.Releases[0].LoadBalancers.Requested != 1 {
		t.Fatalf("load balancers not decoded: %#v", page.Releases[0].LoadBalancers)
	}
	if page.NextCursor != "" {
		t.Fatalf("expected empty cursor, got %q", page.NextCursor)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestServiceListAppReleases_WithCursor(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}

	appSource, _ := json.Marshal(map[string]any{"image": "nginx"})
	mock.ExpectQuery(`SELECT id, project_id, name, status, repo, webhook_secret, external_ref, source FROM apps`).
		WithArgs("app-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "name", "status", "repo", "webhook_secret", "external_ref", "source"}).
			AddRow("app-1", "proj-1", "web", "deployed", nil, nil, nil, appSource))

	specJSON, _ := json.Marshal(map[string]any{"name": "web"})
	renderedJSON, _ := json.Marshal([]map[string]any{{"kind": "Deployment", "name": "web"}})
	lbJSON, _ := json.Marshal(map[string]any{"requested": 1})
	warnJSON, _ := json.Marshal([]string{})
	helmVals, _ := json.Marshal(map[string]any{})
	cursorTime := time.Now()

	cursorRow := sqlmock.NewRows([]string{
		"id", "project_id", "app_id", "source", "spec_digest", "render_digest",
		"spec", "rendered_objects", "load_balancers", "warnings",
		"helm_chart", "helm_values", "helm_render_sha", "manifests_sha", "repo",
		"status", "message", "created_at",
	}).AddRow("cursor", "proj-1", "app-1", "image", "spec", "render", specJSON, renderedJSON, lbJSON, warnJSON, nil, helmVals, nil, nil, nil, "succeeded", "", cursorTime)

	mock.ExpectQuery(`SELECT id, project_id, app_id, source`).
		WithArgs("cursor").
		WillReturnRows(cursorRow)

	listRows := sqlmock.NewRows([]string{
		"id", "project_id", "app_id", "source", "spec_digest", "render_digest",
		"spec", "rendered_objects", "load_balancers", "warnings",
		"helm_chart", "helm_values", "helm_render_sha", "manifests_sha", "repo",
		"status", "message", "created_at",
	}).
		AddRow("rel-3", "proj-1", "app-1", "image", "spec3", "render3", specJSON, renderedJSON, lbJSON, warnJSON, nil, helmVals, nil, nil, nil, "succeeded", "", cursorTime.Add(-time.Minute)).
		AddRow("rel-2", "proj-1", "app-1", "image", "spec2", "render2", specJSON, renderedJSON, lbJSON, warnJSON, nil, helmVals, nil, nil, nil, "succeeded", "", cursorTime.Add(-2*time.Minute)).
		AddRow("rel-1", "proj-1", "app-1", "image", "spec1", "render1", specJSON, renderedJSON, lbJSON, warnJSON, nil, helmVals, nil, nil, nil, "succeeded", "", cursorTime.Add(-3*time.Minute))

	mock.ExpectQuery(`FROM releases WHERE project_id = \$1 AND app_id = \$2 AND \(created_at < \$3 OR \(created_at = \$3 AND id < \$4\)\)`).
		WithArgs("proj-1", "app-1", cursorTime, "cursor", 3).
		WillReturnRows(listRows)

	page, err := svc.ListAppReleases(context.Background(), "proj-1", "app-1", 2, "cursor")
	if err != nil {
		t.Fatalf("ListAppReleases: %v", err)
	}
	if len(page.Releases) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(page.Releases))
	}
	if page.NextCursor != "rel-2" {
		t.Fatalf("expected next cursor rel-2, got %q", page.NextCursor)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestServiceListAppReleases_AppMismatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}

	mock.ExpectQuery(`SELECT id, project_id, name, status, repo, webhook_secret, external_ref, source FROM apps`).
		WithArgs("app-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "name", "status", "repo", "webhook_secret", "external_ref", "source"}).
			AddRow("app-1", "proj-2", "web", "deployed", nil, nil, nil, []byte("{}")))

	if _, err := svc.ListAppReleases(context.Background(), "proj-1", "app-1", 0, ""); err == nil {
		t.Fatal("expected error when project mismatch")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestServiceListAppReleases_InvalidCursor(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}

	appSource, _ := json.Marshal(map[string]any{"image": "nginx"})
	mock.ExpectQuery(`SELECT id, project_id, name, status, repo, webhook_secret, external_ref, source FROM apps`).
		WithArgs("app-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "name", "status", "repo", "webhook_secret", "external_ref", "source"}).
			AddRow("app-1", "proj-1", "web", "deployed", nil, nil, nil, appSource))

	mock.ExpectQuery(`SELECT id, project_id, app_id, source`).
		WithArgs("cursor").
		WillReturnError(sql.ErrNoRows)

	if _, err := svc.ListAppReleases(context.Background(), "proj-1", "app-1", 10, "cursor"); err == nil {
		t.Fatal("expected cursor error")
	} else if !strings.Contains(err.Error(), "invalid cursor") {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestServiceListAppReleases_CursorProjectMismatch(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st := store.NewWithDB(db)
	cfg := &config.Config{KcfgEncryptionKey: "unit-test"}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}

	appSource, _ := json.Marshal(map[string]any{"image": "nginx"})
	mock.ExpectQuery(`SELECT id, project_id, name, status, repo, webhook_secret, external_ref, source FROM apps`).
		WithArgs("app-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "project_id", "name", "status", "repo", "webhook_secret", "external_ref", "source"}).
			AddRow("app-1", "proj-1", "web", "deployed", nil, nil, nil, appSource))

	specJSON, _ := json.Marshal(map[string]any{"name": "web"})
	renderedJSON, _ := json.Marshal([]map[string]any{{"kind": "Deployment", "name": "web"}})
	lbJSON, _ := json.Marshal(map[string]any{"requested": 1})
	warnJSON, _ := json.Marshal([]string{})
	helmVals, _ := json.Marshal(map[string]any{})
	cursorTime := time.Now()

	cursorRow := sqlmock.NewRows([]string{
		"id", "project_id", "app_id", "source", "spec_digest", "render_digest",
		"spec", "rendered_objects", "load_balancers", "warnings",
		"helm_chart", "helm_values", "helm_render_sha", "manifests_sha", "repo",
		"status", "message", "created_at",
	}).AddRow("cursor", "proj-2", "app-1", "image", "spec", "render", specJSON, renderedJSON, lbJSON, warnJSON, nil, helmVals, nil, nil, nil, "succeeded", "", cursorTime)

	mock.ExpectQuery(`SELECT id, project_id, app_id, source`).
		WithArgs("cursor").
		WillReturnRows(cursorRow)

	if _, err := svc.ListAppReleases(context.Background(), "proj-1", "app-1", 10, "cursor"); err == nil {
		t.Fatal("expected project mismatch error")
	} else if !strings.Contains(err.Error(), "cursor does not match project") {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestServiceDeployApp_RecordsRelease(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{
		KcfgEncryptionKey:          "unit-test",
		MaxLoadBalancersPerProject: 5,
		EventsDBEnabled:            true,
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
	baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	svc.SetKubeManager(fakeKM{client: noopPatchClient{Client: baseClient}})
	svc.SetDNSProviderFactory(func(*config.Config) dns.Provider { return nil })

	now := time.Now()
	mock.ExpectQuery(`SELECT id, user_id, cluster_id, name, namespace, suspended, created_at, quota_overrides, kubeconfig_enc FROM projects`).
		WithArgs("proj-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "cluster_id", "name", "namespace", "suspended", "created_at", "quota_overrides", "kubeconfig_enc"}).
			AddRow("proj-1", "user-1", "cluster-1", "Project", "tenant-ns", false, now, []byte("{}"), []byte("enc")))
	mock.ExpectQuery(`SELECT c\.id, c\.name`).
		WithArgs("cluster-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "owner", "contact", "environment", "region", "api_server", "description", "tags",
			"created_at", "last_seen", "status_id", "healthy", "message", "apiserver_version", "node_count",
			"checked_at", "details",
		}).AddRow(
			"cluster-1", "stage", nil, nil, nil, nil, nil, nil, []byte("[]"), now, nil, nil, nil, nil, nil, nil, nil, []byte("{}"),
		))

	mock.ExpectExec(`INSERT INTO apps`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO releases`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`INSERT INTO project_events`).
		WithArgs(sqlmock.AnyArg(), "proj-1", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(now))

	_, err = svc.DeployApp(context.Background(), service.AppDeployInput{
		ProjectID: "proj-1",
		Name:      "web",
		Image:     "nginx:1",
	})
	if err != nil {
		t.Fatalf("DeployApp: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestServiceDeployApp_RecordReleaseError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{
		KcfgEncryptionKey:          "unit-test",
		MaxLoadBalancersPerProject: 5,
		EventsDBEnabled:            true,
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
	baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	svc.SetKubeManager(fakeKM{client: noopPatchClient{Client: baseClient}})
	svc.SetDNSProviderFactory(func(*config.Config) dns.Provider { return nil })

	now := time.Now()
	mock.ExpectQuery(`SELECT id, user_id, cluster_id, name, namespace, suspended, created_at, quota_overrides, kubeconfig_enc FROM projects`).
		WithArgs("proj-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "cluster_id", "name", "namespace", "suspended", "created_at", "quota_overrides", "kubeconfig_enc"}).
			AddRow("proj-1", "user-1", "cluster-1", "Project", "tenant-ns", false, now, []byte("{}"), []byte("enc")))
	mock.ExpectQuery(`SELECT c\.id, c\.name`).
		WithArgs("cluster-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "owner", "contact", "environment", "region", "api_server", "description", "tags",
			"created_at", "last_seen", "status_id", "healthy", "message", "apiserver_version", "node_count",
			"checked_at", "details",
		}).AddRow(
			"cluster-1", "stage", nil, nil, nil, nil, nil, nil, []byte("[]"), now, nil, nil, nil, nil, nil, nil, nil, []byte("{}"),
		))

	mock.ExpectExec(`INSERT INTO apps`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO releases`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnError(errors.New("store failed"))

	if _, err := svc.DeployApp(context.Background(), service.AppDeployInput{
		ProjectID: "proj-1",
		Name:      "web",
		Image:     "nginx:1",
	}); err == nil {
		t.Fatal("expected deploy error when release insert fails")
	} else if !strings.Contains(err.Error(), "record release") {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestServiceDeployApp_GitManifestsRecordsRelease(t *testing.T) {
	repoDir := writeGitRepo(t, map[string]string{
		"configmap.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: sample\ndata:\n  key: value\n",
	})
	repoURL := "file://" + repoDir

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{
		KcfgEncryptionKey:          "unit-test",
		MaxLoadBalancersPerProject: 3,
		EventsDBEnabled:            true,
		AllowGitFileProtocol:       true,
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
	baseClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	svc.SetKubeManager(fakeKM{client: noopPatchClient{Client: baseClient}})
	svc.SetDNSProviderFactory(func(*config.Config) dns.Provider { return nil })

	now := time.Now()
	mock.ExpectQuery(`SELECT id, user_id, cluster_id, name, namespace, suspended, created_at, quota_overrides, kubeconfig_enc FROM projects`).
		WithArgs("proj-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "cluster_id", "name", "namespace", "suspended", "created_at", "quota_overrides", "kubeconfig_enc"}).
			AddRow("proj-1", "user-1", "cluster-1", "Project", "tenant-ns", false, now, []byte("{}"), []byte("enc")))
	mock.ExpectQuery(`SELECT c\.id, c\.name`).
		WithArgs("cluster-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "owner", "contact", "environment", "region", "api_server", "description", "tags",
			"created_at", "last_seen", "status_id", "healthy", "message", "apiserver_version", "node_count",
			"checked_at", "details",
		}).AddRow(
			"cluster-1", "stage", nil, nil, nil, nil, nil, nil, []byte("[]"), now, nil, nil, nil, nil, nil, nil, nil, []byte("{}"),
		))

	mock.ExpectExec(`INSERT INTO apps`).
		WithArgs(sqlmock.AnyArg(), "proj-1", "git-app", "deployed", repoURL, "", sqlmock.AnyArg(), jsonContains(`"git"`)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO releases`).
		WithArgs(
			sqlmock.AnyArg(),
			"proj-1",
			sqlmock.AnyArg(),
			"git:manifests",
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			jsonContains(`"git"`),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
			sql.NullString{String: repoURL, Valid: true},
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`INSERT INTO project_events`).
		WithArgs(sqlmock.AnyArg(), "proj-1", sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"at"}).AddRow(now))

	out, err := svc.DeployApp(context.Background(), service.AppDeployInput{
		ProjectID: "proj-1",
		Name:      "git-app",
		Git: &service.AppGitSpec{
			URL: repoURL,
			Ref: "refs/heads/master",
		},
	})
	if err != nil {
		t.Fatalf("DeployApp returned error: %v", err)
	}
	if out.AppID == "" {
		t.Fatalf("expected app id in deploy output")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

type noopPatchClient struct {
	client.Client
}

func (n noopPatchClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	return nil
}

type jsonContains string

func (j jsonContains) Match(v driver.Value) bool {
	switch val := v.(type) {
	case []byte:
		return strings.Contains(string(val), string(j))
	case string:
		return strings.Contains(val, string(j))
	default:
		return false
	}
}
