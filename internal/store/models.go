package store

import (
	"time"
)

type User struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
}

type Cluster struct {
	ID                     string     `json:"id"`
	Name                   string     `json:"name"`
	CreatedAt              time.Time  `json:"created_at"`
	WatcherStatus          string     `json:"watcher_status"`
	WatcherStatusMessage   *string    `json:"watcher_status_message,omitempty"`
	WatcherStatusUpdatedAt time.Time  `json:"watcher_status_updated_at"`
	WatcherReadyAt         *time.Time `json:"watcher_ready_at,omitempty"`
	WatcherHealthDeadline  time.Time  `json:"watcher_health_deadline"`
}

type Project struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	ClusterID string    `json:"cluster_id"`
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	Suspended bool      `json:"suspended"`
	CreatedAt time.Time `json:"created_at"`
}

type UserSpace struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	ClusterID string    `json:"cluster_id"`
	Namespace string    `json:"namespace"`
	CreatedAt time.Time `json:"created_at"`
}

type KubeconfigRecord struct {
	ID             string    `json:"id"`
	ClusterID      string    `json:"cluster_id"`
	Namespace      string    `json:"namespace"`
	UserID         string    `json:"user_id"`
	ProjectID      *string   `json:"project_id,omitempty"`
	ServiceAccount string    `json:"service_account"`
	SecretName     string    `json:"secret_name"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
