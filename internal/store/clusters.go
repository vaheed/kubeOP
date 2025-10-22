package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

func (s *Store) CreateCluster(ctx context.Context, c Cluster, kubeconfigEnc []byte) (Cluster, error) {
	c.Tags = normalizeTags(c.Tags)
	tagsJSON, err := json.Marshal(c.Tags)
	if err != nil {
		return Cluster{}, err
	}
	const q = `INSERT INTO clusters (id, name, owner, contact, environment, region, api_server, description, tags, kubeconfig_enc) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING created_at`
	if err := s.db.QueryRowContext(ctx, q,
		c.ID,
		c.Name,
		nullableString(c.Owner),
		nullableString(c.Contact),
		nullableString(c.Environment),
		nullableString(c.Region),
		nullableString(c.APIServer),
		nullableString(c.Description),
		tagsJSON,
		kubeconfigEnc,
	).Scan(&c.CreatedAt); err != nil {
		return Cluster{}, err
	}
	return c, nil
}

func (s *Store) UpdateClusterMetadata(ctx context.Context, c Cluster) (Cluster, error) {
	c.Tags = normalizeTags(c.Tags)
	tagsJSON, err := json.Marshal(c.Tags)
	if err != nil {
		return Cluster{}, err
	}
	const q = `UPDATE clusters SET owner = $2, contact = $3, environment = $4, region = $5, api_server = $6, description = $7, tags = $8 WHERE id = $1`
	res, err := s.db.ExecContext(ctx, q,
		c.ID,
		nullableString(c.Owner),
		nullableString(c.Contact),
		nullableString(c.Environment),
		nullableString(c.Region),
		nullableString(c.APIServer),
		nullableString(c.Description),
		tagsJSON,
	)
	if err != nil {
		return Cluster{}, err
	}
	if rows, err := res.RowsAffected(); err == nil && rows == 0 {
		return Cluster{}, sql.ErrNoRows
	}
	updated, err := s.GetCluster(ctx, c.ID)
	if err != nil {
		return Cluster{}, err
	}
	return updated, nil
}

func (s *Store) ListClusters(ctx context.Context) ([]Cluster, error) {
	const q = `
SELECT c.id, c.name, c.owner, c.contact, c.environment, c.region, c.api_server, c.description, c.tags, c.created_at, c.last_seen,
       s.id, s.healthy, s.message, s.apiserver_version, s.node_count, s.checked_at, s.details
FROM clusters c
LEFT JOIN cluster_status s ON s.id = c.last_status_id
ORDER BY c.created_at DESC`
	rows, err := s.db.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Cluster
	for rows.Next() {
		cluster, err := scanCluster(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, cluster)
	}
	return out, rows.Err()
}

func (s *Store) GetClusterKubeconfigEnc(ctx context.Context, id string) ([]byte, error) {
	const q = `SELECT kubeconfig_enc FROM clusters WHERE id = $1`
	var b []byte
	if err := s.db.QueryRowContext(ctx, q, id).Scan(&b); err != nil {
		return nil, err
	}
	return b, nil
}

func (s *Store) GetCluster(ctx context.Context, id string) (Cluster, error) {
	const q = `
SELECT c.id, c.name, c.owner, c.contact, c.environment, c.region, c.api_server, c.description, c.tags, c.created_at, c.last_seen,
       s.id, s.healthy, s.message, s.apiserver_version, s.node_count, s.checked_at, s.details
FROM clusters c
LEFT JOIN cluster_status s ON s.id = c.last_status_id
WHERE c.id = $1`
	return scanCluster(s.db.QueryRowContext(ctx, q, id))
}

// DeleteCluster removes a cluster record. Primarily used for rollbacks when provisioning fails.
func (s *Store) DeleteCluster(ctx context.Context, id string) error {
	const q = `DELETE FROM clusters WHERE id = $1`
	_, err := s.db.ExecContext(ctx, q, id)
	return err
}

func (s *Store) ListClusterStatus(ctx context.Context, clusterID string, limit int) ([]ClusterStatus, error) {
	if limit <= 0 {
		limit = 20
	}
	const q = `SELECT id, cluster_id, healthy, message, apiserver_version, node_count, checked_at, details FROM cluster_status WHERE cluster_id = $1 ORDER BY checked_at DESC LIMIT $2`
	rows, err := s.db.QueryContext(ctx, q, clusterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ClusterStatus
	for rows.Next() {
		st, err := scanClusterStatus(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

func (s *Store) InsertClusterStatus(ctx context.Context, st ClusterStatus) (ClusterStatus, error) {
	if st.CheckedAt.IsZero() {
		st.CheckedAt = time.Now().UTC()
	}
	if st.Details == nil {
		st.Details = map[string]any{}
	}
	detailsJSON, err := json.Marshal(st.Details)
	if err != nil {
		return ClusterStatus{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ClusterStatus{}, err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()
	var checkedAt time.Time
	const insert = `INSERT INTO cluster_status (id, cluster_id, healthy, message, apiserver_version, node_count, checked_at, details) VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING checked_at`
	if err = tx.QueryRowContext(ctx, insert,
		st.ID,
		st.ClusterID,
		st.Healthy,
		nullableString(st.Message),
		nullableString(st.APIServerVersion),
		nullableInt(st.NodeCount),
		st.CheckedAt,
		detailsJSON,
	).Scan(&checkedAt); err != nil {
		return ClusterStatus{}, err
	}
	st.CheckedAt = checkedAt
	const update = `UPDATE clusters SET last_status_id = $2, last_seen = $3 WHERE id = $1`
	if _, err = tx.ExecContext(ctx, update, st.ClusterID, st.ID, st.CheckedAt); err != nil {
		return ClusterStatus{}, err
	}
	if err = tx.Commit(); err != nil {
		return ClusterStatus{}, err
	}
	return st, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCluster(row rowScanner) (Cluster, error) {
	var (
		c                                                           Cluster
		owner, contact, environment, region, apiServer, description sql.NullString
		tagsRaw                                                     []byte
		lastSeen                                                    sql.NullTime
		statusID, statusMessage, statusVersion                      sql.NullString
		statusHealthy                                               sql.NullBool
		statusNodeCount                                             sql.NullInt64
		statusChecked                                               sql.NullTime
		statusDetails                                               []byte
	)
	scanErr := row.Scan(
		&c.ID,
		&c.Name,
		&owner,
		&contact,
		&environment,
		&region,
		&apiServer,
		&description,
		&tagsRaw,
		&c.CreatedAt,
		&lastSeen,
		&statusID,
		&statusHealthy,
		&statusMessage,
		&statusVersion,
		&statusNodeCount,
		&statusChecked,
		&statusDetails,
	)
	if scanErr != nil {
		return Cluster{}, scanErr
	}
	if owner.Valid {
		c.Owner = owner.String
	}
	if contact.Valid {
		c.Contact = contact.String
	}
	if environment.Valid {
		c.Environment = environment.String
	}
	if region.Valid {
		c.Region = region.String
	}
	if apiServer.Valid {
		c.APIServer = apiServer.String
	}
	if description.Valid {
		c.Description = description.String
	}
	if len(tagsRaw) > 0 {
		if err := json.Unmarshal(tagsRaw, &c.Tags); err != nil {
			return Cluster{}, err
		}
	}
	c.Tags = normalizeTags(c.Tags)
	if lastSeen.Valid {
		ts := lastSeen.Time
		c.LastSeen = &ts
	}
	if statusID.Valid {
		cs := &ClusterStatus{
			ID:        statusID.String,
			ClusterID: c.ID,
			Healthy:   statusHealthy.Bool,
		}
		if statusMessage.Valid {
			cs.Message = statusMessage.String
		}
		if statusVersion.Valid {
			cs.APIServerVersion = statusVersion.String
		}
		if statusNodeCount.Valid {
			n := int(statusNodeCount.Int64)
			cs.NodeCount = &n
		}
		if statusChecked.Valid {
			cs.CheckedAt = statusChecked.Time
		}
		if len(statusDetails) > 0 {
			var details map[string]any
			if err := json.Unmarshal(statusDetails, &details); err == nil {
				cs.Details = details
			}
		}
		c.LastStatus = cs
	}
	return c, nil
}

func scanClusterStatus(row rowScanner) (ClusterStatus, error) {
	var (
		st               ClusterStatus
		message, version sql.NullString
		nodeCount        sql.NullInt64
		detailsRaw       []byte
	)
	if err := row.Scan(
		&st.ID,
		&st.ClusterID,
		&st.Healthy,
		&message,
		&version,
		&nodeCount,
		&st.CheckedAt,
		&detailsRaw,
	); err != nil {
		return ClusterStatus{}, err
	}
	if message.Valid {
		st.Message = message.String
	}
	if version.Valid {
		st.APIServerVersion = version.String
	}
	if nodeCount.Valid {
		n := int(nodeCount.Int64)
		st.NodeCount = &n
	}
	if len(detailsRaw) > 0 {
		var details map[string]any
		if err := json.Unmarshal(detailsRaw, &details); err == nil {
			st.Details = details
		}
	}
	return st, nil
}

func normalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(tags))
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		cleaned := strings.TrimSpace(t)
		if cleaned == "" {
			continue
		}
		cleaned = strings.ToLower(cleaned)
		if _, ok := seen[cleaned]; ok {
			continue
		}
		seen[cleaned] = struct{}{}
		out = append(out, cleaned)
	}
	sort.Strings(out)
	return out
}

func nullableString(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return v
}

func nullableInt(v *int) any {
	if v == nil {
		return nil
	}
	return *v
}
