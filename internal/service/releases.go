package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"kubeop/internal/logging"
	"kubeop/internal/store"
)

type ReleaseSource struct {
	Type      string            `json:"type"`
	Image     string            `json:"image,omitempty"`
	Helm      map[string]any    `json:"helm,omitempty"`
	Manifests []string          `json:"manifests,omitempty"`
	Git       *ReleaseGitSource `json:"git,omitempty"`
}

type ReleaseGitSource struct {
	URL          string `json:"url"`
	Ref          string `json:"ref,omitempty"`
	Commit       string `json:"commit,omitempty"`
	Path         string `json:"path,omitempty"`
	Mode         string `json:"mode,omitempty"`
	CredentialID string `json:"credentialId,omitempty"`
}

type ReleaseSpec struct {
	Name      string            `json:"name"`
	KubeName  string            `json:"kubeName"`
	Flavor    string            `json:"flavor,omitempty"`
	Resources map[string]string `json:"resources,omitempty"`
	Replicas  int32             `json:"replicas"`
	Env       map[string]string `json:"env,omitempty"`
	Secrets   []string          `json:"secrets,omitempty"`
	Ports     []AppPort         `json:"ports,omitempty"`
	Domain    string            `json:"domain,omitempty"`
	Host      string            `json:"host,omitempty"`
	Repo      string            `json:"repo,omitempty"`
	Source    ReleaseSource     `json:"source"`
}

type AppRelease struct {
	ID              string                  `json:"id"`
	ProjectID       string                  `json:"projectId"`
	AppID           string                  `json:"appId"`
	CreatedAt       time.Time               `json:"createdAt"`
	Source          string                  `json:"source"`
	SpecDigest      string                  `json:"specDigest"`
	RenderDigest    string                  `json:"renderDigest"`
	Repo            string                  `json:"repo,omitempty"`
	Spec            ReleaseSpec             `json:"spec"`
	RenderedObjects []RenderedObjectSummary `json:"renderedObjects"`
	LoadBalancers   LoadBalancerSummary     `json:"loadBalancers"`
	Warnings        []string                `json:"warnings,omitempty"`
	HelmChart       string                  `json:"helmChart,omitempty"`
	HelmValues      map[string]any          `json:"helmValues,omitempty"`
	HelmRenderSHA   string                  `json:"helmRenderSha,omitempty"`
	ManifestsSHA    string                  `json:"manifestsSha,omitempty"`
	Status          string                  `json:"status"`
	Message         string                  `json:"message,omitempty"`
}

type AppReleasePage struct {
	Releases   []AppRelease `json:"releases"`
	NextCursor string       `json:"nextCursor,omitempty"`
}

func (s *Service) recordRelease(ctx context.Context, plan *appDeploymentPlan, input AppDeployInput) (string, error) {
	if s == nil || s.st == nil {
		return "", errors.New("service not initialised")
	}
	spec := ReleaseSpec{
		Name:      input.Name,
		KubeName:  plan.KubeName,
		Flavor:    plan.Flavor,
		Resources: cloneStringMap(plan.Resources),
		Replicas:  plan.Replicas,
		Env:       cloneStringMap(plan.Env),
		Secrets:   cloneStringSlice(plan.Secrets),
		Ports:     clonePorts(plan.Ports),
		Domain:    plan.Domain,
		Host:      plan.Host,
		Repo:      plan.Repo,
		Source:    ReleaseSource{Type: plan.SourceType},
	}
	switch plan.SourceType {
	case "image":
		spec.Source.Image = plan.Image
	case "manifests":
		spec.Source.Manifests = cloneStringSlice(plan.Manifests)
	case "helm":
		spec.Source.Helm = cloneAnyMap(plan.HelmSpec)
	default:
		if plan.Git != nil && strings.HasPrefix(plan.SourceType, "git:") {
			spec.Source.Manifests = cloneStringSlice(plan.Manifests)
			spec.Source.Git = &ReleaseGitSource{
				URL:          plan.Git.URL,
				Ref:          plan.Git.Ref,
				Commit:       plan.Git.Commit,
				Path:         plan.Git.Path,
				Mode:         plan.Git.Mode,
				CredentialID: plan.Git.CredentialID,
			}
		}
	}
	specMap, specDigest, err := encodeForRelease(spec)
	if err != nil {
		return "", err
	}
	renderedObjectsMap, err := encodeRenderedObjects(plan.RenderedObjs)
	if err != nil {
		return "", err
	}
	loadBalancers := map[string]any{
		"requested": plan.LBSummary.Requested,
		"existing":  plan.LBSummary.Existing,
		"limit":     plan.LBSummary.Limit,
	}
	warnings := append([]string(nil), plan.Warnings...)
	helmValues := cloneAnyMap(plan.HelmValues)
	helmRenderSHA := hashIfNotEmpty(plan.HelmRendered)
	manifestsSHA := hashSliceIfNotEmpty(plan.Manifests)
	renderFingerprint := map[string]any{
		"renderedObjects": plan.RenderedObjs,
	}
	if plan.HelmChart != "" {
		renderFingerprint["helmChart"] = plan.HelmChart
	}
	if len(plan.HelmValues) > 0 {
		renderFingerprint["helmValues"] = plan.HelmValues
	}
	if helmRenderSHA != "" {
		renderFingerprint["helmRenderSha"] = helmRenderSHA
	}
	if manifestsSHA != "" {
		renderFingerprint["manifestsSha"] = manifestsSHA
	}
	_, renderDigest, err := encodeForRelease(renderFingerprint)
	if err != nil {
		return "", err
	}
	releaseID := uuid.New().String()
	release := store.Release{
		ID:              releaseID,
		ProjectID:       plan.Project.ID,
		AppID:           plan.AppID,
		Source:          plan.SourceType,
		SpecDigest:      specDigest,
		RenderDigest:    renderDigest,
		Spec:            specMap,
		RenderedObjects: renderedObjectsMap,
		LoadBalancers:   loadBalancers,
		Warnings:        warnings,
		HelmValues:      helmValues,
		Status:          "succeeded",
	}
	if plan.HelmChart != "" {
		release.HelmChart = &plan.HelmChart
	}
	if helmRenderSHA != "" {
		release.HelmRenderSHA = &helmRenderSHA
	}
	if manifestsSHA != "" {
		release.ManifestsSHA = &manifestsSHA
	}
	if strings.TrimSpace(plan.Repo) != "" {
		repo := strings.TrimSpace(plan.Repo)
		release.Repo = &repo
	}
	if len(warnings) == 0 {
		release.Warnings = []string{}
	}
	if helmValues == nil {
		release.HelmValues = map[string]any{}
	}
	if err := s.st.CreateRelease(ctx, release); err != nil {
		logging.AppErrorLogger(plan.Project.ID, plan.AppID).Error("release_store_failed", zap.Error(err))
		return "", err
	}
	return releaseID, nil
}

func (s *Service) ListAppReleases(ctx context.Context, projectID, appID string, limit int, cursor string) (AppReleasePage, error) {
	if s == nil || s.st == nil {
		return AppReleasePage{}, errors.New("service not initialised")
	}
	projectID = strings.TrimSpace(projectID)
	appID = strings.TrimSpace(appID)
	if projectID == "" || appID == "" {
		return AppReleasePage{}, errors.New("projectId and appId are required")
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	app, err := s.st.GetApp(ctx, appID)
	if err != nil {
		return AppReleasePage{}, err
	}
	if app.ProjectID != projectID {
		return AppReleasePage{}, errors.New("app does not belong to project")
	}
	var cur store.ReleaseCursor
	cursor = strings.TrimSpace(cursor)
	if cursor != "" {
		rel, err := s.st.GetRelease(ctx, cursor)
		if err != nil {
			return AppReleasePage{}, fmt.Errorf("invalid cursor: %w", err)
		}
		if rel.ProjectID != projectID {
			return AppReleasePage{}, errors.New("cursor does not match project")
		}
		if rel.AppID != appID {
			return AppReleasePage{}, errors.New("cursor does not match app")
		}
		cur = store.ReleaseCursor{ID: rel.ID, CreatedAt: rel.CreatedAt}
	}
	rows, err := s.st.ListReleasesByApp(ctx, projectID, appID, limit+1, cur)
	if err != nil {
		return AppReleasePage{}, err
	}
	page := AppReleasePage{Releases: make([]AppRelease, 0, len(rows))}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	for _, row := range rows {
		converted, err := convertRelease(row)
		if err != nil {
			return AppReleasePage{}, err
		}
		page.Releases = append(page.Releases, converted)
	}
	if hasMore {
		last := rows[len(rows)-1]
		page.NextCursor = last.ID
	}
	return page, nil
}

func encodeForRelease(payload any) (map[string]any, string, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, "", err
	}
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, "", err
	}
	digest := sha256.Sum256(data)
	return decoded, hex.EncodeToString(digest[:]), nil
}

func encodeRenderedObjects(objs []RenderedObjectSummary) ([]map[string]any, error) {
	data, err := json.Marshal(objs)
	if err != nil {
		return nil, err
	}
	var decoded []map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

func hashIfNotEmpty(value string) string {
	if strings.TrimSpace(value) == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func hashSliceIfNotEmpty(values []string) string {
	if len(values) == 0 {
		return ""
	}
	data, err := json.Marshal(values)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func convertRelease(rel store.Release) (AppRelease, error) {
	specJSON, err := json.Marshal(rel.Spec)
	if err != nil {
		return AppRelease{}, fmt.Errorf("encode spec: %w", err)
	}
	var spec ReleaseSpec
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		return AppRelease{}, fmt.Errorf("decode spec: %w", err)
	}
	objsJSON, err := json.Marshal(rel.RenderedObjects)
	if err != nil {
		return AppRelease{}, fmt.Errorf("encode rendered objects: %w", err)
	}
	rendered := []RenderedObjectSummary{}
	if len(objsJSON) > 0 {
		if err := json.Unmarshal(objsJSON, &rendered); err != nil {
			return AppRelease{}, fmt.Errorf("decode rendered objects: %w", err)
		}
	}
	lbSummary := decodeLoadBalancers(rel.LoadBalancers)
	warnings := append([]string(nil), rel.Warnings...)
	helmValues := cloneAnyMap(rel.HelmValues)
	release := AppRelease{
		ID:              rel.ID,
		ProjectID:       rel.ProjectID,
		AppID:           rel.AppID,
		CreatedAt:       rel.CreatedAt,
		Source:          rel.Source,
		SpecDigest:      rel.SpecDigest,
		RenderDigest:    rel.RenderDigest,
		Spec:            spec,
		RenderedObjects: rendered,
		LoadBalancers:   lbSummary,
		Warnings:        warnings,
		HelmValues:      helmValues,
		Status:          strings.TrimSpace(rel.Status),
		Message:         strings.TrimSpace(rel.Message),
	}
	if rel.Repo != nil {
		release.Repo = *rel.Repo
	}
	if rel.HelmChart != nil {
		release.HelmChart = *rel.HelmChart
	}
	if rel.HelmRenderSHA != nil {
		release.HelmRenderSHA = *rel.HelmRenderSHA
	}
	if rel.ManifestsSHA != nil {
		release.ManifestsSHA = *rel.ManifestsSHA
	}
	if release.Status == "" {
		release.Status = "succeeded"
	}
	if release.HelmValues == nil {
		release.HelmValues = map[string]any{}
	}
	return release, nil
}

func decodeLoadBalancers(in map[string]any) LoadBalancerSummary {
	summary := LoadBalancerSummary{}
	if v, ok := toInt(in["requested"]); ok {
		summary.Requested = v
	}
	if v, ok := toInt(in["existing"]); ok {
		summary.Existing = v
	}
	if v, ok := toInt(in["limit"]); ok {
		summary.Limit = v
	}
	return summary
}

func toInt(v any) (int, bool) {
	switch tv := v.(type) {
	case int:
		return tv, true
	case int32:
		return int(tv), true
	case int64:
		return int(tv), true
	case float64:
		return int(tv), true
	case float32:
		return int(tv), true
	default:
		return 0, false
	}
}
