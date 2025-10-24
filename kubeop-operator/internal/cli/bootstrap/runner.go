package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/yaml"
)

const (
	defaultFieldOwner = "bootstrap.kubeop.io/cli"
	defaultOutDir     = "out"
)

// Runner applies Kubernetes resources and records audit events plus filesystem outputs.
type Runner struct {
	client     client.Client
	scheme     *runtime.Scheme
	logger     *zap.SugaredLogger
	events     EventSink
	fieldOwner string
	outDir     string
}

// ApplySummary captures the identifying fields for an applied manifest.
type ApplySummary struct {
	Group     string
	Version   string
	Kind      string
	Name      string
	Namespace string
}

// NewRunner validates dependencies and constructs a Runner instance.
func NewRunner(c client.Client, scheme *runtime.Scheme, logger *zap.SugaredLogger, sink EventSink, outDir string) (*Runner, error) {
	if c == nil {
		return nil, fmt.Errorf("client is required")
	}
	if scheme == nil {
		return nil, fmt.Errorf("scheme is required")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger is required")
	}
	if sink == nil {
		return nil, fmt.Errorf("event sink is required")
	}
	if outDir == "" {
		outDir = defaultOutDir
	}
	return &Runner{
		client:     c,
		scheme:     scheme,
		logger:     logger,
		events:     sink,
		fieldOwner: defaultFieldOwner,
		outDir:     outDir,
	}, nil
}

// ApplyObject ensures the object exists on the cluster, emits an audit event, and writes the manifest to disk.
func (r *Runner) ApplyObject(ctx context.Context, obj client.Object) (ApplySummary, error) {
	if r == nil {
		return ApplySummary{}, fmt.Errorf("runner is nil")
	}
	if obj == nil {
		return ApplySummary{}, fmt.Errorf("object is required")
	}
	gvk, err := apiutil.GVKForObject(obj, r.scheme)
	if err != nil {
		return ApplySummary{}, fmt.Errorf("determine gvk: %w", err)
	}
	if err := ApplyObject(ctx, r.client, r.scheme, obj, r.fieldOwner); err != nil {
		return ApplySummary{}, err
	}
	summary := ApplySummary{
		Group:     gvk.Group,
		Version:   gvk.Version,
		Kind:      gvk.Kind,
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
	}
	if err := r.writeObject(obj, gvk); err != nil {
		return ApplySummary{}, err
	}
	if err := r.events.Emit(ctx, NewAuditEvent("io.kubeop.bootstrap.apply", "cli/bootstrap", summary)); err != nil {
		r.logger.Warnw("Failed to emit CloudEvent", "error", err)
	}
	r.logger.Infow("Applied Kubernetes object", "kind", gvk.Kind, "name", obj.GetName(), "namespace", obj.GetNamespace())
	return summary, nil
}

func (r *Runner) writeObject(obj client.Object, gvk schema.GroupVersionKind) error {
	if err := os.MkdirAll(r.outDir, 0o755); err != nil {
		return fmt.Errorf("ensure out dir %s: %w", r.outDir, err)
	}
	obj.GetObjectKind().SetGroupVersionKind(gvk)
	data, err := r.encodeYAML(obj)
	if err != nil {
		return err
	}
	filename := fmt.Sprintf("%s_%s.yaml", strings.ToLower(gvk.Kind), sanitizeFilename(obj.GetName()))
	path := filepath.Join(r.outDir, filename)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write manifest %s: %w", path, err)
	}
	return nil
}

func sanitizeFilename(name string) string {
	if name == "" {
		return "resource"
	}
	replacer := strings.NewReplacer("/", "-", "\\", "-", " ", "-")
	return replacer.Replace(name)
}

// EncodeYAML renders the object into YAML using the runner scheme.
func (r *Runner) EncodeYAML(obj runtime.Object) ([]byte, error) {
	if r == nil {
		return nil, fmt.Errorf("runner is nil")
	}
	return r.encodeYAML(obj)
}

func (r *Runner) encodeYAML(obj runtime.Object) ([]byte, error) {
	serializer := json.NewSerializerWithOptions(json.DefaultMetaFactory, r.scheme, r.scheme, json.SerializerOptions{Yaml: true, Pretty: true})
	var builder strings.Builder
	if err := serializer.Encode(obj, &builder); err != nil {
		data, marshalErr := yaml.Marshal(obj)
		if marshalErr != nil {
			return nil, fmt.Errorf("serialise object: %w / fallback: %v", err, marshalErr)
		}
		return data, nil
	}
	return []byte(builder.String()), nil
}

// SummariesToTable converts applied summaries into a table-style slice of strings for output.
func SummariesToTable(summaries []ApplySummary) [][]string {
	headers := []string{"KIND", "NAME", "NAMESPACE", "GROUP", "VERSION"}
	rows := make([][]string, 0, len(summaries)+1)
	rows = append(rows, headers)
	for _, summary := range summaries {
		rows = append(rows, []string{
			summary.Kind,
			summary.Name,
			summary.Namespace,
			summary.Group,
			summary.Version,
		})
	}
	return rows
}
