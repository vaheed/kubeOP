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
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Owner       string         `json:"owner,omitempty"`
	Contact     string         `json:"contact,omitempty"`
	Environment string         `json:"environment,omitempty"`
	Region      string         `json:"region,omitempty"`
	APIServer   string         `json:"apiServer,omitempty"`
	Description string         `json:"description,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	LastSeen    *time.Time     `json:"lastSeen,omitempty"`
	LastStatus  *ClusterStatus `json:"lastStatus,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

type ClusterStatus struct {
	ID               string         `json:"id"`
	ClusterID        string         `json:"clusterId"`
	Healthy          bool           `json:"healthy"`
	Message          string         `json:"message,omitempty"`
	APIServerVersion string         `json:"apiServerVersion,omitempty"`
	NodeCount        *int           `json:"nodeCount,omitempty"`
	CheckedAt        time.Time      `json:"checkedAt"`
	Details          map[string]any `json:"details,omitempty"`
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

type Release struct {
	ID              string           `json:"id"`
	ProjectID       string           `json:"projectId"`
	AppID           string           `json:"appId"`
	Source          string           `json:"source"`
	SpecDigest      string           `json:"specDigest"`
	RenderDigest    string           `json:"renderDigest"`
	Spec            map[string]any   `json:"spec"`
	RenderedObjects []map[string]any `json:"renderedObjects"`
	LoadBalancers   map[string]any   `json:"loadBalancers"`
	Warnings        []string         `json:"warnings,omitempty"`
	HelmChart       *string          `json:"helmChart,omitempty"`
	HelmValues      map[string]any   `json:"helmValues,omitempty"`
	HelmRenderSHA   *string          `json:"helmRenderSha,omitempty"`
	ManifestsSHA    *string          `json:"manifestsSha,omitempty"`
	Repo            *string          `json:"repo,omitempty"`
	SBOM            map[string]any   `json:"sbom,omitempty"`
	Status          string           `json:"status"`
	Message         string           `json:"message,omitempty"`
	CreatedAt       time.Time        `json:"created_at"`
}

type AppTemplate struct {
	ID         string         `json:"id"`
	AppID      string         `json:"appId"`
	TemplateID string         `json:"templateId"`
	Values     map[string]any `json:"values"`
	Metadata   map[string]any `json:"metadata"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
}

type ReleaseCursor struct {
	ID        string
	CreatedAt time.Time
}

type MaintenanceState struct {
	ID        string    `json:"id"`
	Enabled   bool      `json:"enabled"`
	Message   string    `json:"message"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy string    `json:"updated_by"`
}
