package bootstrapassets

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed kustomize/bases/crds/*.yaml kustomize/bases/rbac/*.yaml kustomize/bases/webhooks/*.yaml kustomize/bases/defaults/*.yaml
var content embed.FS

// CRDs exposes the embedded CRD manifests.
func CRDs() (fs.FS, error) {
	return subdir("kustomize/bases/crds")
}

// RBAC exposes the embedded RBAC manifests.
func RBAC() (fs.FS, error) {
	return subdir("kustomize/bases/rbac")
}

// Webhooks exposes the embedded webhook manifests.
func Webhooks() (fs.FS, error) {
	return subdir("kustomize/bases/webhooks")
}

// Defaults exposes the embedded default objects.
func Defaults() (fs.FS, error) {
	return subdir("kustomize/bases/defaults")
}

func subdir(path string) (fs.FS, error) {
	sub, err := fs.Sub(content, path)
	if err != nil {
		return nil, fmt.Errorf("open embedded directory %s: %w", path, err)
	}
	return sub, nil
}
