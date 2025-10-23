package bootstrap

import _ "embed"

//go:embed crd/kubeop.io_apps.yaml
var appCRDManifest []byte
