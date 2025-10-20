package testcase

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"kubeop/internal/store"
)

func TestStoreCreateCluster_InsertsMetadata(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	cluster := store.Cluster{
		ID:          "cluster-1",
		Name:        "prod-east",
		Owner:       "platform",
		Environment: "prod",
		Region:      "us-east",
		Tags:        []string{"Prod", "core", "prod"},
	}

	now := time.Now().UTC()
	mock.ExpectQuery(`INSERT INTO clusters`).
		WithArgs(
			cluster.ID,
			cluster.Name,
			cluster.Owner,
			nil,
			cluster.Environment,
			cluster.Region,
			nil,
			nil,
			sqlmock.AnyArg(),
			[]byte("enc"),
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).AddRow(now))

	created, err := st.CreateCluster(context.Background(), cluster, []byte("enc"))
	if err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}
	if created.CreatedAt.IsZero() {
		t.Fatalf("expected created_at to be set")
	}
	if len(created.Tags) != 2 || created.Tags[0] != "core" || created.Tags[1] != "prod" {
		t.Fatalf("expected normalized tags, got %#v", created.Tags)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestStoreListClusters_ScansMetadataAndStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	tagsJSON, _ := json.Marshal([]string{"prod", "core"})
	detailsJSON, _ := json.Marshal(map[string]any{"stage": "listNamespaces"})
	now := time.Now().UTC()

	rows := sqlmock.NewRows([]string{
		"id", "name", "owner", "contact", "environment", "region", "api_server", "description", "tags", "created_at", "last_seen",
		"status_id", "healthy", "message", "apiserver_version", "node_count", "checked_at", "details",
	}).
		AddRow(
			"cluster-1",
			"prod-east",
			"platform",
			"",
			"prod",
			"us-east",
			"https://example",
			"primary",
			tagsJSON,
			now,
			now,
			"status-1",
			true,
			"connected",
			"v1.30.0",
			nil,
			now,
			detailsJSON,
		)

	mock.ExpectQuery(`SELECT c.id, c.name`).WillReturnRows(rows)

	clusters, err := st.ListClusters(context.Background())
	if err != nil {
		t.Fatalf("ListClusters: %v", err)
	}
	if len(clusters) != 1 {
		t.Fatalf("expected 1 cluster, got %d", len(clusters))
	}
	c := clusters[0]
	if c.Owner != "platform" || c.Environment != "prod" || c.APIServer != "https://example" {
		t.Fatalf("unexpected metadata: %+v", c)
	}
	if c.LastStatus == nil || !c.LastStatus.Healthy || c.LastStatus.Message != "connected" {
		t.Fatalf("expected last status, got %#v", c.LastStatus)
	}
	if len(c.Tags) != 2 {
		t.Fatalf("expected 2 tags, got %#v", c.Tags)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestStoreInsertClusterStatus_PersistsAndUpdates(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	st := store.NewWithDB(db)
	status := store.ClusterStatus{
		ID:        "status-1",
		ClusterID: "cluster-1",
		Healthy:   true,
		Message:   "connected",
		Details:   map[string]any{"stage": "listNamespaces"},
	}

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO cluster_status`).
		WithArgs(
			status.ID,
			status.ClusterID,
			status.Healthy,
			status.Message,
			nil,
			nil,
			sqlmock.AnyArg(),
			sqlmock.AnyArg(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"checked_at"}).AddRow(time.Now().UTC()))
	mock.ExpectExec(`UPDATE clusters SET last_status_id =`).
		WithArgs(status.ClusterID, status.ID, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	saved, err := st.InsertClusterStatus(context.Background(), status)
	if err != nil {
		t.Fatalf("InsertClusterStatus: %v", err)
	}
	if saved.ID != status.ID || saved.ClusterID != status.ClusterID {
		t.Fatalf("unexpected saved status: %#v", saved)
	}
	if saved.CheckedAt.IsZero() {
		t.Fatalf("expected checked_at to be populated")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
