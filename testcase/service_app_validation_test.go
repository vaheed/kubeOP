package testcase

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"kubeop/internal/config"
	"kubeop/internal/crypto"
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

func newFakeClient(t *testing.T) client.Client {
	t.Helper()
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
	return fake.NewClientBuilder().WithScheme(scheme).Build()
}

func writeGitRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	repo, err := git.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("git init: %v", err)
	}
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}
	for name, content := range files {
		abs := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
			t.Fatalf("write file: %v", err)
		}
		if _, err := wt.Add(name); err != nil {
			t.Fatalf("git add %s: %v", name, err)
		}
	}
	_, err = wt.Commit("initial", &git.CommitOptions{Author: &object.Signature{Name: "Tester", Email: "tester@example.com", When: time.Now()}})
	if err != nil {
		t.Fatalf("git commit: %v", err)
	}
	return dir
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
	disableMaintenance(t, svc)

	svc.SetKubeManager(fakeKM{client: newFakeClient(t)})

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

func TestServiceValidateApp_HelmOCI_Public(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{
		KcfgEncryptionKey:          "unit-test",
		MaxLoadBalancersPerProject: 2,
		PaaSWildcardEnabled:        false,
	}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)

	svc.SetKubeManager(fakeKM{client: newFakeClient(t)})

	now := time.Now()
	mock.ExpectQuery(`SELECT id, user_id, cluster_id, name, namespace, suspended, created_at, quota_overrides, kubeconfig_enc FROM projects WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs("proj-oci").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "cluster_id", "name", "namespace", "suspended", "created_at", "quota_overrides", "kubeconfig_enc"}).
			AddRow("proj-oci", "user-1", "cluster-1", "Project OCI", "tenant-ns", false, now, []byte("{}"), []byte("enc")))
	mock.ExpectQuery(`SELECT c\.id, c\.name`).
		WithArgs("cluster-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "owner", "contact", "environment", "region", "api_server", "description", "tags",
			"created_at", "last_seen", "status_id", "healthy", "message", "apiserver_version", "node_count",
			"checked_at", "details",
		}).AddRow(
			"cluster-1", "stage", nil, nil, nil, nil, nil, nil, []byte("[]"), now, nil, nil, nil, nil, nil, nil, nil, []byte("{}"),
		))

	chartBytes := buildTestHelmChartArchive(t)
	stub := &stubRegistryClient{expectedRef: "oci://registry.example.com/library/chart:1.0.0", chartBytes: chartBytes}
	restoreResolver := service.SetHelmChartHostResolver(func(ctx context.Context, host string) ([]net.IP, error) {
		if host != "registry.example.com" {
			return nil, fmt.Errorf("unexpected host: %s", host)
		}
		return []net.IP{net.ParseIP("198.51.100.30")}, nil
	})
	t.Cleanup(restoreResolver)
	restoreFactory := service.SetHelmRegistryClientFactory(func(host string, addrs []netip.Addr, insecure bool) (service.HelmRegistryClient, error) {
		return stub, nil
	})
	t.Cleanup(restoreFactory)

	out, err := svc.ValidateApp(context.Background(), service.AppDeployInput{
		ProjectID: "proj-oci",
		Name:      "helm-app",
		Helm: map[string]any{
			"oci": map[string]any{
				"ref": "oci://registry.example.com/library/chart:1.0.0",
			},
			"values": map[string]any{"replicaCount": 1},
		},
	})
	if err != nil {
		t.Fatalf("ValidateApp returned error: %v", err)
	}
	if out.Source != "helm" {
		t.Fatalf("expected source helm, got %s", out.Source)
	}
	if out.HelmChart != "oci://registry.example.com/library/chart:1.0.0" {
		t.Fatalf("unexpected helm chart %s", out.HelmChart)
	}
	if stub.pullCalls != 1 {
		t.Fatalf("expected pull call, got %d", stub.pullCalls)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestServiceValidateApp_OCIBundle(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{
		KcfgEncryptionKey:          "unit-test",
		MaxLoadBalancersPerProject: 1,
	}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)
	svc.SetKubeManager(fakeKM{client: newFakeClient(t)})

	bundleRef := "oci://registry.example.com/bundles/sample:1.0.0"
	restore := service.SetOCIBundleFetcher(func(ctx context.Context, ref string, insecure bool, auth *service.OCIRegistryAuth) (service.OCIBundleFetchResult, error) {
		if ref != bundleRef {
			t.Fatalf("unexpected ref %s", ref)
		}
		return service.OCIBundleFetchResult{
			Documents: []string{"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: bundle\n"},
			Digest:    "sha256:abcdef",
			MediaType: "application/vnd.kubeop.bundle.v1+tar",
		}, nil
	})
	t.Cleanup(restore)

	now := time.Now()
	mock.ExpectQuery(`SELECT id, user_id, cluster_id, name, namespace, suspended, created_at, quota_overrides, kubeconfig_enc FROM projects WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs("proj-oci").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "cluster_id", "name", "namespace", "suspended", "created_at", "quota_overrides", "kubeconfig_enc"}).
			AddRow("proj-oci", "user-1", "cluster-1", "Project OCI", "tenant-ns", false, now, []byte("{}"), []byte("enc")))
	mock.ExpectQuery(`SELECT c\.id, c\.name`).
		WithArgs("cluster-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "owner", "contact", "environment", "region", "api_server", "description", "tags",
			"created_at", "last_seen", "status_id", "healthy", "message", "apiserver_version", "node_count",
			"checked_at", "details",
		}).AddRow(
			"cluster-1", "stage", nil, nil, nil, nil, nil, nil, []byte("[]"), now, nil, nil, nil, nil, nil, nil, nil, []byte("{}"),
		))

	out, err := svc.ValidateApp(context.Background(), service.AppDeployInput{
		ProjectID: "proj-oci",
		Name:      "bundle",
		OciBundle: &service.AppOCIBundleSpec{Ref: bundleRef},
	})
	if err != nil {
		t.Fatalf("ValidateApp returned error: %v", err)
	}
	if out.Source != "ociBundle" {
		t.Fatalf("expected source ociBundle, got %s", out.Source)
	}
	if out.OciBundleRef != bundleRef {
		t.Fatalf("expected bundle ref %s, got %s", bundleRef, out.OciBundleRef)
	}
	if out.OciBundleDigest != "sha256:abcdef" {
		t.Fatalf("expected digest sha256:abcdef, got %s", out.OciBundleDigest)
	}
	if len(out.RenderedObjects) == 0 {
		t.Fatalf("expected rendered objects")
	}
	if out.RenderedObjects[0].Kind != "ConfigMap" {
		t.Fatalf("expected ConfigMap rendered, got %#v", out.RenderedObjects)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestServiceValidateApp_HelmOCI_RegistryCredential(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{
		KcfgEncryptionKey:          "unit-test",
		MaxLoadBalancersPerProject: 2,
	}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)

	svc.SetKubeManager(fakeKM{client: newFakeClient(t)})

	now := time.Now()
	mock.ExpectQuery(`SELECT id, user_id, cluster_id, name, namespace, suspended, created_at, quota_overrides, kubeconfig_enc FROM projects WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs("proj-cred").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "cluster_id", "name", "namespace", "suspended", "created_at", "quota_overrides", "kubeconfig_enc"}).
			AddRow("proj-cred", "user-2", "cluster-1", "Project Cred", "tenant-ns", false, now, []byte("{}"), []byte("enc")))
	mock.ExpectQuery(`SELECT c\.id, c\.name`).
		WithArgs("cluster-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "owner", "contact", "environment", "region", "api_server", "description", "tags",
			"created_at", "last_seen", "status_id", "healthy", "message", "apiserver_version", "node_count",
			"checked_at", "details",
		}).AddRow(
			"cluster-1", "stage", nil, nil, nil, nil, nil, nil, []byte("[]"), now, nil, nil, nil, nil, nil, nil, nil, []byte("{}"),
		))

	encKey := crypto.DeriveKey(cfg.KcfgEncryptionKey)
	secretPayload := []byte(`{"password":"s3cret"}`)
	encSecret, err := crypto.EncryptAESGCM(secretPayload, encKey)
	if err != nil {
		t.Fatalf("encrypt secret: %v", err)
	}
	mock.ExpectQuery(`SELECT id, name, registry, user_id, project_id, auth_type, username, secret_enc, created_at, updated_at FROM registry_credentials WHERE id = \$1`).
		WithArgs("cred-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "registry", "user_id", "project_id", "auth_type", "username", "secret_enc", "created_at", "updated_at"}).
			AddRow("cred-1", "dockerhub", "https://registry.example.com", nil, "proj-cred", "BASIC", "ci", encSecret, now, now))

	chartBytes := buildTestHelmChartArchive(t)
	stub := &stubRegistryClient{expectedRef: "oci://registry.example.com/team/chart:3.0.0", chartBytes: chartBytes}
	restoreResolver := service.SetHelmChartHostResolver(func(ctx context.Context, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("198.51.100.40")}, nil
	})
	t.Cleanup(restoreResolver)
	restoreFactory := service.SetHelmRegistryClientFactory(func(host string, addrs []netip.Addr, insecure bool) (service.HelmRegistryClient, error) {
		return stub, nil
	})
	t.Cleanup(restoreFactory)

	out, err := svc.ValidateApp(context.Background(), service.AppDeployInput{
		ProjectID: "proj-cred",
		Name:      "helm-app",
		Helm: map[string]any{
			"oci": map[string]any{
				"ref":                  "oci://registry.example.com/team/chart:3.0.0",
				"registryCredentialId": "cred-1",
			},
		},
	})
	if err != nil {
		t.Fatalf("ValidateApp returned error: %v", err)
	}
	if stub.loginCalls != 1 {
		t.Fatalf("expected login call, got %d", stub.loginCalls)
	}
	if out.HelmChart != "oci://registry.example.com/team/chart:3.0.0" {
		t.Fatalf("unexpected helm chart %s", out.HelmChart)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestServiceValidateApp_HelmOCI_RegistryHostMismatch(t *testing.T) {
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
	disableMaintenance(t, svc)

	svc.SetKubeManager(fakeKM{client: newFakeClient(t)})

	now := time.Now()
	mock.ExpectQuery(`SELECT id, user_id, cluster_id, name, namespace, suspended, created_at, quota_overrides, kubeconfig_enc FROM projects WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs("proj-mismatch").
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "cluster_id", "name", "namespace", "suspended", "created_at", "quota_overrides", "kubeconfig_enc"}).
			AddRow("proj-mismatch", "user-3", "cluster-1", "Project Mismatch", "tenant-ns", false, now, []byte("{}"), []byte("enc")))
	mock.ExpectQuery(`SELECT c\.id, c\.name`).
		WithArgs("cluster-1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "owner", "contact", "environment", "region", "api_server", "description", "tags",
			"created_at", "last_seen", "status_id", "healthy", "message", "apiserver_version", "node_count",
			"checked_at", "details",
		}).AddRow(
			"cluster-1", "stage", nil, nil, nil, nil, nil, nil, []byte("[]"), now, nil, nil, nil, nil, nil, nil, nil, []byte("{}"),
		))

	encSecret, err := crypto.EncryptAESGCM([]byte(`{"password":"s3cret"}`), crypto.DeriveKey(cfg.KcfgEncryptionKey))
	if err != nil {
		t.Fatalf("encrypt secret: %v", err)
	}
	mock.ExpectQuery(`SELECT id, name, registry, user_id, project_id, auth_type, username, secret_enc, created_at, updated_at FROM registry_credentials WHERE id = \$1`).
		WithArgs("cred-2").
		WillReturnRows(sqlmock.NewRows([]string{"id", "name", "registry", "user_id", "project_id", "auth_type", "username", "secret_enc", "created_at", "updated_at"}).
			AddRow("cred-2", "other", "https://other.example.com", nil, "proj-mismatch", "BASIC", "ci", encSecret, now, now))

	_, err = svc.ValidateApp(context.Background(), service.AppDeployInput{
		ProjectID: "proj-mismatch",
		Name:      "helm-app",
		Helm: map[string]any{
			"oci": map[string]any{
				"ref":                  "oci://registry.example.com/team/chart:1.0.0",
				"registryCredentialId": "cred-2",
			},
		},
	})
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected host mismatch error, got %v", err)
	}
}

func TestServiceValidateApp_GitManifests(t *testing.T) {
	repoDir := writeGitRepo(t, map[string]string{
		"deploy.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: sample\n  namespace: tenant-ns\ndata:\n  key: value\n",
	})

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{
		KcfgEncryptionKey:          "unit-test",
		MaxLoadBalancersPerProject: 1,
		AllowGitFileProtocol:       true,
	}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)
	svc.SetKubeManager(fakeKM{client: newFakeClient(t)})

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

	out, err := svc.ValidateApp(context.Background(), service.AppDeployInput{
		ProjectID: "proj-1",
		Name:      "cm",
		Git: &service.AppGitSpec{
			URL: "file://" + repoDir,
			Ref: "refs/heads/master",
		},
	})
	if err != nil {
		t.Fatalf("ValidateApp returned error: %v", err)
	}
	if out.Source != "git:manifests" {
		t.Fatalf("expected git:manifests source, got %s", out.Source)
	}
	if out.GitRepo == "" || out.GitCommit == "" {
		t.Fatalf("expected git metadata, got repo=%q commit=%q", out.GitRepo, out.GitCommit)
	}
	if len(out.RenderedObjects) == 0 {
		t.Fatalf("expected rendered objects for git manifests")
	}
}

func TestServiceValidateApp_GitKustomize(t *testing.T) {
	repoDir := writeGitRepo(t, map[string]string{
		"kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n- deployment.yaml\n",
		"deployment.yaml":    "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web\nspec:\n  selector:\n    matchLabels:\n      app: web\n  template:\n    metadata:\n      labels:\n        app: web\n    spec:\n      containers:\n      - name: web\n        image: nginx\n",
	})

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cfg := &config.Config{
		KcfgEncryptionKey:          "unit-test",
		MaxLoadBalancersPerProject: 1,
		AllowGitFileProtocol:       true,
	}
	svc, err := service.New(cfg, st, nil)
	if err != nil {
		t.Fatalf("service.New: %v", err)
	}
	disableMaintenance(t, svc)
	svc.SetKubeManager(fakeKM{client: newFakeClient(t)})

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

	out, err := svc.ValidateApp(context.Background(), service.AppDeployInput{
		ProjectID: "proj-1",
		Name:      "web",
		Git: &service.AppGitSpec{
			URL:  "file://" + repoDir,
			Ref:  "refs/heads/master",
			Mode: "kustomize",
		},
	})
	if err != nil {
		t.Fatalf("ValidateApp returned error: %v", err)
	}
	if out.Source != "git:kustomize" {
		t.Fatalf("expected git:kustomize source, got %s", out.Source)
	}
	if out.GitCommit == "" {
		t.Fatalf("expected git commit metadata")
	}
	foundDeployment := false
	for _, ro := range out.RenderedObjects {
		if ro.Kind == "Deployment" {
			foundDeployment = true
			break
		}
	}
	if !foundDeployment {
		t.Fatalf("expected deployment in rendered objects, got %#v", out.RenderedObjects)
	}
}
