package bootstrap

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"sort"
	"strings"

	bootstrapassets "github.com/vaheed/kubeOP/kubeop-operator"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// LoadRBACManifests returns embedded RBAC resources.
func LoadRBACManifests() ([]client.Object, error) {
	f, err := bootstrapassets.RBAC()
	if err != nil {
		return nil, err
	}
	return loadManifests(f)
}

// LoadWebhookManifests returns embedded webhook resources.
func LoadWebhookManifests() ([]client.Object, error) {
	f, err := bootstrapassets.Webhooks()
	if err != nil {
		return nil, err
	}
	return loadManifests(f)
}

// LoadDefaultManifests returns embedded default resources.
func LoadDefaultManifests() ([]client.Object, error) {
	f, err := bootstrapassets.Defaults()
	if err != nil {
		return nil, err
	}
	return loadManifests(f)
}

func loadManifests(fsys fs.FS) ([]client.Object, error) {
	entries, err := fs.Glob(fsys, "*.yaml")
	if err != nil {
		return nil, fmt.Errorf("list manifests: %w", err)
	}
	sort.Strings(entries)
	objects := make([]client.Object, 0, len(entries))
	for _, entry := range entries {
		content, err := fs.ReadFile(fsys, entry)
		if err != nil {
			return nil, fmt.Errorf("read manifest %s: %w", entry, err)
		}
		docs, err := decodeDocuments(content)
		if err != nil {
			return nil, fmt.Errorf("decode manifest %s: %w", entry, err)
		}
		objects = append(objects, docs...)
	}
	return objects, nil
}

func decodeDocuments(data []byte) ([]client.Object, error) {
	decoder := yamlutil.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	var objects []client.Object
	for {
		var raw map[string]any
		err := decoder.Decode(&raw)
		switch {
		case err == nil:
			if len(raw) == 0 {
				continue
			}
			obj := &unstructured.Unstructured{Object: raw}
			if strings.TrimSpace(obj.GetKind()) == "" || strings.TrimSpace(obj.GetAPIVersion()) == "" {
				continue
			}
			objects = append(objects, obj)
		case err == io.EOF:
			return objects, nil
		default:
			return nil, err
		}
	}
}
