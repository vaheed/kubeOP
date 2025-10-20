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

type GitCredential struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	UserID    *string   `json:"user_id,omitempty"`
	ProjectID *string   `json:"project_id,omitempty"`
	AuthType  string    `json:"auth_type"`
	Username  string    `json:"username,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type RegistryCredential struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Registry  string    `json:"registry"`
	UserID    *string   `json:"user_id,omitempty"`
	ProjectID *string   `json:"project_id,omitempty"`
	AuthType  string    `json:"auth_type"`
	Username  string    `json:"username,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Template struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Kind             string         `json:"kind"`
	Description      string         `json:"description"`
	Schema           map[string]any `json:"schema"`
	Defaults         map[string]any `json:"defaults"`
	Example          map[string]any `json:"example,omitempty"`
	Base             map[string]any `json:"base,omitempty"`
	DeliveryTemplate string         `json:"deliveryTemplate"`
	CreatedAt        time.Time      `json:"created_at"`
}
