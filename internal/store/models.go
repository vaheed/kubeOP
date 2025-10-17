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
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
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

type Watcher struct {
	ID                    string     `json:"id"`
	ClusterID             string     `json:"cluster_id"`
	RefreshTokenHash      string     `json:"-"`
	RefreshTokenExpiresAt time.Time  `json:"refresh_token_expires_at"`
	AccessTokenExpiresAt  time.Time  `json:"access_token_expires_at"`
	LastSeenAt            *time.Time `json:"last_seen_at,omitempty"`
	LastRefreshAt         *time.Time `json:"last_refresh_at,omitempty"`
	CreatedAt             time.Time  `json:"created_at"`
	UpdatedAt             time.Time  `json:"updated_at"`
	Disabled              bool       `json:"disabled"`
}
