package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/google/uuid"
	"github.com/santhosh-tekuri/jsonschema/v5"
	"go.uber.org/zap"
	"kubeop/internal/store"
	"sigs.k8s.io/yaml"
)

// TemplateCreateInput captures the metadata and rendering blueprint required to register
// a reusable application template.
type TemplateCreateInput struct {
	Name             string
	Kind             string
	Description      string
	Schema           map[string]any
	Defaults         map[string]any
	Example          map[string]any
	Base             map[string]any
	DeliveryTemplate string
}

// TemplateSummary provides high-level catalog metadata for templates.
type TemplateSummary struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Kind        string    `json:"kind"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"createdAt"`
}

// TemplateDetail exposes the full template definition including schema and defaults.
type TemplateDetail struct {
	TemplateSummary
	Schema           map[string]any `json:"schema"`
	Defaults         map[string]any `json:"defaults"`
	Example          map[string]any `json:"example,omitempty"`
	Base             map[string]any `json:"base,omitempty"`
	DeliveryTemplate string         `json:"deliveryTemplate"`
}

// TemplateRenderedApp describes the deploy payload produced after instantiating a template.
type TemplateRenderedApp struct {
	Name          string            `json:"name"`
	Flavor        string            `json:"flavor,omitempty"`
	Resources     map[string]string `json:"resources,omitempty"`
	Replicas      *int32            `json:"replicas,omitempty"`
	Env           map[string]string `json:"env,omitempty"`
	Secrets       []string          `json:"secrets,omitempty"`
	Ports         []AppPort         `json:"ports,omitempty"`
	Domain        string            `json:"domain,omitempty"`
	Repo          string            `json:"repo,omitempty"`
	WebhookSecret string            `json:"webhookSecret,omitempty"`
	Image         string            `json:"image,omitempty"`
	Helm          map[string]any    `json:"helm,omitempty"`
	Manifests     []string          `json:"manifests,omitempty"`
}

// TemplateRenderOutput captures the result of rendering a template alongside the values
// that were validated and merged.
type TemplateRenderOutput struct {
	Template TemplateSummary     `json:"template"`
	Values   map[string]any      `json:"values"`
	App      TemplateRenderedApp `json:"app"`
}

// CreateTemplate validates and stores a template blueprint with JSON Schema enforcement.
func (s *Service) CreateTemplate(ctx context.Context, in TemplateCreateInput) (TemplateDetail, error) {
	if strings.TrimSpace(in.Name) == "" {
		return TemplateDetail{}, errors.New("name is required")
	}
	if strings.TrimSpace(in.Kind) == "" {
		return TemplateDetail{}, errors.New("kind is required")
	}
	if strings.TrimSpace(in.Description) == "" {
		return TemplateDetail{}, errors.New("description is required")
	}
	if len(in.Schema) == 0 {
		return TemplateDetail{}, errors.New("schema is required")
	}
	if len(in.Defaults) == 0 {
		return TemplateDetail{}, errors.New("defaults are required")
	}
	if strings.TrimSpace(in.DeliveryTemplate) == "" {
		return TemplateDetail{}, errors.New("deliveryTemplate is required")
	}

	schema, err := compileSchema(in.Schema)
	if err != nil {
		return TemplateDetail{}, fmt.Errorf("compile schema: %w", err)
	}
	if err := schema.Validate(in.Defaults); err != nil {
		return TemplateDetail{}, fmt.Errorf("defaults do not satisfy schema: %w", err)
	}
	if len(in.Example) > 0 {
		if err := schema.Validate(in.Example); err != nil {
			return TemplateDetail{}, fmt.Errorf("example does not satisfy schema: %w", err)
		}
	}

	tpl := store.Template{
		ID:               uuid.New().String(),
		Name:             strings.TrimSpace(in.Name),
		Kind:             strings.TrimSpace(in.Kind),
		Description:      strings.TrimSpace(in.Description),
		Schema:           cloneMap(in.Schema),
		Defaults:         cloneMap(in.Defaults),
		Example:          cloneMap(in.Example),
		Base:             cloneMap(in.Base),
		DeliveryTemplate: in.DeliveryTemplate,
	}

	// Perform a dry render using defaults to ensure the template produces a usable payload.
	if _, _, err := s.renderTemplate(ctx, tpl, nil); err != nil {
		return TemplateDetail{}, fmt.Errorf("render defaults: %w", err)
	}

	if err := s.st.CreateTemplate(ctx, tpl); err != nil {
		return TemplateDetail{}, err
	}
	stored, err := s.st.GetTemplate(ctx, tpl.ID)
	if err != nil {
		return TemplateDetail{}, err
	}
	s.logger.Info(
		"template_created",
		zap.String("template_id", tpl.ID),
		zap.String("template_name", tpl.Name),
		zap.String("kind", tpl.Kind),
	)
	return TemplateDetail{
		TemplateSummary: TemplateSummary{
			ID:          stored.ID,
			Name:        stored.Name,
			Kind:        stored.Kind,
			Description: stored.Description,
			CreatedAt:   stored.CreatedAt,
		},
		Schema:           stored.Schema,
		Defaults:         stored.Defaults,
		Example:          stored.Example,
		Base:             stored.Base,
		DeliveryTemplate: stored.DeliveryTemplate,
	}, nil
}

// ListTemplates returns published templates ordered by creation time.
func (s *Service) ListTemplates(ctx context.Context) ([]TemplateSummary, error) {
	records, err := s.st.ListTemplates(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]TemplateSummary, 0, len(records))
	for _, tpl := range records {
		out = append(out, TemplateSummary{
			ID:          tpl.ID,
			Name:        tpl.Name,
			Kind:        tpl.Kind,
			Description: tpl.Description,
			CreatedAt:   tpl.CreatedAt,
		})
	}
	return out, nil
}

// GetTemplate returns a detailed template definition.
func (s *Service) GetTemplate(ctx context.Context, id string) (TemplateDetail, error) {
	tpl, err := s.st.GetTemplate(ctx, id)
	if err != nil {
		return TemplateDetail{}, err
	}
	return TemplateDetail{
		TemplateSummary: TemplateSummary{
			ID:          tpl.ID,
			Name:        tpl.Name,
			Kind:        tpl.Kind,
			Description: tpl.Description,
			CreatedAt:   tpl.CreatedAt,
		},
		Schema:           tpl.Schema,
		Defaults:         tpl.Defaults,
		Example:          tpl.Example,
		Base:             tpl.Base,
		DeliveryTemplate: tpl.DeliveryTemplate,
	}, nil
}

// RenderTemplate merges user values with defaults, validates them against the template
// schema, and returns the rendered application payload.
func (s *Service) RenderTemplate(ctx context.Context, id string, values map[string]any) (TemplateRenderOutput, error) {
	tpl, err := s.st.GetTemplate(ctx, id)
	if err != nil {
		return TemplateRenderOutput{}, err
	}
	render, _, err := s.renderTemplate(ctx, tpl, values)
	return render, err
}

// DeployTemplate instantiates a template and deploys the resulting application spec into
// the requested project.
func (s *Service) DeployTemplate(ctx context.Context, projectID, templateID string, values map[string]any) (AppDeployOutput, error) {
	tpl, err := s.st.GetTemplate(ctx, templateID)
	if err != nil {
		return AppDeployOutput{}, err
	}
	render, appInput, err := s.renderTemplate(ctx, tpl, values)
	if err != nil {
		return AppDeployOutput{}, err
	}
	appInput.ProjectID = projectID
	if s.deployAppFn == nil {
		return AppDeployOutput{}, errors.New("deploy function not configured")
	}
	out, err := s.deployAppFn(ctx, appInput)
	if err != nil {
		return AppDeployOutput{}, err
	}
	s.logger.Info(
		"template_deployed",
		zap.String("template_id", tpl.ID),
		zap.String("project_id", projectID),
		zap.String("app_name", render.App.Name),
	)
	return out, nil
}

func (s *Service) renderTemplate(ctx context.Context, tpl store.Template, input map[string]any) (TemplateRenderOutput, AppDeployInput, error) {
	_ = ctx
	schema, err := compileSchema(tpl.Schema)
	if err != nil {
		return TemplateRenderOutput{}, AppDeployInput{}, fmt.Errorf("compile schema: %w", err)
	}
	mergedValues := mergeMaps(cloneMap(tpl.Defaults), input)
	if err := schema.Validate(mergedValues); err != nil {
		return TemplateRenderOutput{}, AppDeployInput{}, fmt.Errorf("values do not satisfy schema: %w", err)
	}
	tmpl, err := template.New("delivery").Funcs(templateFuncMap()).Parse(tpl.DeliveryTemplate)
	if err != nil {
		return TemplateRenderOutput{}, AppDeployInput{}, fmt.Errorf("parse delivery template: %w", err)
	}
	var buf bytes.Buffer
	data := map[string]any{
		"values":   mergedValues,
		"defaults": tpl.Defaults,
		"base":     tpl.Base,
		"schema":   tpl.Schema,
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		return TemplateRenderOutput{}, AppDeployInput{}, fmt.Errorf("execute delivery template: %w", err)
	}
	payload := strings.TrimSpace(buf.String())
	if payload == "" {
		return TemplateRenderOutput{}, AppDeployInput{}, errors.New("delivery template rendered empty payload")
	}
	rawJSON, err := yaml.YAMLToJSON([]byte(payload))
	if err != nil {
		return TemplateRenderOutput{}, AppDeployInput{}, fmt.Errorf("render to json: %w", err)
	}
	specMap := map[string]any{}
	if err := json.Unmarshal(rawJSON, &specMap); err != nil {
		return TemplateRenderOutput{}, AppDeployInput{}, fmt.Errorf("decode rendered payload: %w", err)
	}
	mergedSpec := mergeMaps(cloneMap(tpl.Base), specMap)
	specJSON, err := json.Marshal(mergedSpec)
	if err != nil {
		return TemplateRenderOutput{}, AppDeployInput{}, fmt.Errorf("encode merged payload: %w", err)
	}
	var rendered TemplateRenderedApp
	if err := json.Unmarshal(specJSON, &rendered); err != nil {
		return TemplateRenderOutput{}, AppDeployInput{}, fmt.Errorf("decode rendered app: %w", err)
	}
	if strings.TrimSpace(rendered.Name) == "" {
		return TemplateRenderOutput{}, AppDeployInput{}, errors.New("rendered spec missing name")
	}
	if strings.TrimSpace(rendered.Image) == "" && len(rendered.Helm) == 0 && len(rendered.Manifests) == 0 {
		return TemplateRenderOutput{}, AppDeployInput{}, errors.New("rendered spec missing delivery source (image/helm/manifests)")
	}
	var appInput AppDeployInput
	if err := json.Unmarshal(specJSON, &appInput); err != nil {
		return TemplateRenderOutput{}, AppDeployInput{}, fmt.Errorf("decode app input: %w", err)
	}
	output := TemplateRenderOutput{
		Template: TemplateSummary{
			ID:          tpl.ID,
			Name:        tpl.Name,
			Kind:        tpl.Kind,
			Description: tpl.Description,
			CreatedAt:   tpl.CreatedAt,
		},
		Values: mergedValues,
		App:    rendered,
	}
	return output, appInput, nil
}

func templateFuncMap() template.FuncMap {
	fn := sprig.TxtFuncMap()
	fn["toJSON"] = func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(b)
	}
	return fn
}

func compileSchema(doc map[string]any) (*jsonschema.Schema, error) {
	data, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("template.json", bytes.NewReader(data)); err != nil {
		return nil, err
	}
	return c.Compile("template.json")
}

func cloneMap(in map[string]any) map[string]any {
	if len(in) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = cloneValue(v)
	}
	return out
}

func cloneSlice(in []any) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = cloneValue(v)
	}
	return out
}

func cloneValue(v any) any {
	switch val := v.(type) {
	case map[string]any:
		return cloneMap(val)
	case []any:
		return cloneSlice(val)
	default:
		return val
	}
}

func mergeMaps(base map[string]any, overlay map[string]any) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	out := cloneMap(base)
	if len(overlay) == 0 {
		return out
	}
	for k, v := range overlay {
		switch ov := v.(type) {
		case map[string]any:
			if bv, ok := out[k].(map[string]any); ok {
				out[k] = mergeMaps(bv, ov)
				continue
			}
			out[k] = cloneMap(ov)
		case []any:
			out[k] = cloneSlice(ov)
		default:
			out[k] = ov
		}
	}
	return out
}
