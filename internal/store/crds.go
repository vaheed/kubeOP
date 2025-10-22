package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type K8sCRD struct {
	ID              string         `json:"id"`
	ClusterID       string         `json:"cluster_id"`
	ProjectID       string         `json:"project_id"`
	Kind            string         `json:"kind"`
	Namespace       string         `json:"namespace"`
	Name            string         `json:"name"`
	UID             string         `json:"uid"`
	ResourceVersion string         `json:"resource_version"`
	SpecHash        string         `json:"spec_hash"`
	Spec            map[string]any `json:"spec"`
	Status          map[string]any `json:"status"`
	DeletedAt       sql.NullTime   `json:"deleted_at"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
}

func (s *Store) UpsertK8sCRD(ctx context.Context, in K8sCRD) (K8sCRD, error) {
	if strings.TrimSpace(in.ClusterID) == "" || strings.TrimSpace(in.ProjectID) == "" || strings.TrimSpace(in.Kind) == "" || strings.TrimSpace(in.Namespace) == "" || strings.TrimSpace(in.Name) == "" {
		return K8sCRD{}, fmt.Errorf("clusterID, projectID, kind, namespace, and name are required")
	}
	if strings.TrimSpace(in.UID) == "" || strings.TrimSpace(in.ResourceVersion) == "" || strings.TrimSpace(in.SpecHash) == "" {
		return K8sCRD{}, fmt.Errorf("uid, resourceVersion, and specHash are required")
	}
	if in.Spec == nil {
		in.Spec = map[string]any{}
	}
	if in.Status == nil {
		in.Status = map[string]any{}
	}
	id := strings.TrimSpace(in.ID)
	if id == "" {
		id = uuid.New().String()
	}
	specJSON, err := json.Marshal(in.Spec)
	if err != nil {
		return K8sCRD{}, err
	}
	statusJSON, err := json.Marshal(in.Status)
	if err != nil {
		return K8sCRD{}, err
	}
	const q = `INSERT INTO k8s_crds (id, cluster_id, project_id, kind, namespace, name, uid, resource_version, spec_hash, spec, status, deleted_at)
VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
ON CONFLICT (cluster_id, kind, namespace, name)
DO UPDATE SET
        uid = EXCLUDED.uid,
        resource_version = EXCLUDED.resource_version,
        spec_hash = EXCLUDED.spec_hash,
        spec = EXCLUDED.spec,
        status = EXCLUDED.status,
        deleted_at = EXCLUDED.deleted_at,
        updated_at = NOW()
RETURNING id, cluster_id, project_id, kind, namespace, name, uid, resource_version, spec_hash, spec, status, deleted_at, created_at, updated_at`
	var out K8sCRD
	var specOut, statusOut []byte
	if err := s.db.QueryRowContext(ctx, q, id, in.ClusterID, in.ProjectID, in.Kind, in.Namespace, in.Name, in.UID, in.ResourceVersion, in.SpecHash, specJSON, statusJSON, in.DeletedAt).Scan(
		&out.ID,
		&out.ClusterID,
		&out.ProjectID,
		&out.Kind,
		&out.Namespace,
		&out.Name,
		&out.UID,
		&out.ResourceVersion,
		&out.SpecHash,
		&specOut,
		&statusOut,
		&out.DeletedAt,
		&out.CreatedAt,
		&out.UpdatedAt,
	); err != nil {
		return K8sCRD{}, err
	}
	_ = json.Unmarshal(specOut, &out.Spec)
	_ = json.Unmarshal(statusOut, &out.Status)
	return out, nil
}

func (s *Store) MarkK8sCRDDeleted(ctx context.Context, clusterID, kind, namespace, name string, deletedAt time.Time) error {
	if deletedAt.IsZero() {
		deletedAt = time.Now().UTC()
	}
	const q = `UPDATE k8s_crds SET deleted_at = $5, updated_at = NOW() WHERE cluster_id = $1 AND kind = $2 AND namespace = $3 AND name = $4`
	_, err := s.db.ExecContext(ctx, q, clusterID, kind, namespace, name, deletedAt)
	return err
}
