package watch

import (
	"sort"
	"strings"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Kind describes a supported Kubernetes resource watch target.
type Kind struct {
	Name string
	GVR  schema.GroupVersionResource
}

var kindAliases map[string]Kind

var kindDefinitions = []struct {
	name    string
	aliases []string
	gvr     schema.GroupVersionResource
}{
	{
		name:    "Pod",
		aliases: []string{"pod", "pods"},
		gvr:     schema.GroupVersionResource{Group: "", Version: "v1", Resource: "pods"},
	},
	{
		name:    "Deployment",
		aliases: []string{"deployment", "deployments"},
		gvr:     schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
	},
	{
		name:    "Service",
		aliases: []string{"service", "services"},
		gvr:     schema.GroupVersionResource{Group: "", Version: "v1", Resource: "services"},
	},
	{
		name:    "Ingress",
		aliases: []string{"ingress", "ingresses"},
		gvr:     schema.GroupVersionResource{Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
	},
	{
		name:    "Job",
		aliases: []string{"job", "jobs"},
		gvr:     schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"},
	},
	{
		name:    "CronJob",
		aliases: []string{"cronjob", "cronjobs"},
		gvr:     schema.GroupVersionResource{Group: "batch", Version: "v1", Resource: "cronjobs"},
	},
	{
		name:    "HorizontalPodAutoscaler",
		aliases: []string{"horizontalpodautoscaler", "hpa", "hpas"},
		gvr:     schema.GroupVersionResource{Group: "autoscaling", Version: "v2", Resource: "horizontalpodautoscalers"},
	},
	{
		name:    "PersistentVolumeClaim",
		aliases: []string{"persistentvolumeclaim", "pvc", "pvcs"},
		gvr:     schema.GroupVersionResource{Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
	},
	{
		name:    "ConfigMap",
		aliases: []string{"configmap", "configmaps"},
		gvr:     schema.GroupVersionResource{Group: "", Version: "v1", Resource: "configmaps"},
	},
	{
		name:    "Secret",
		aliases: []string{"secret", "secrets"},
		gvr:     schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"},
	},
	{
		name:    "Event",
		aliases: []string{"event", "events"},
		gvr:     schema.GroupVersionResource{Group: "", Version: "v1", Resource: "events"},
	},
	{
		name:    "Certificate",
		aliases: []string{"certificate", "certificates"},
		gvr:     schema.GroupVersionResource{Group: "cert-manager.io", Version: "v1", Resource: "certificates"},
	},
}

func init() {
	kindAliases = make(map[string]Kind)
	for _, def := range kindDefinitions {
		kind := Kind{Name: def.name, GVR: def.gvr}
		for _, alias := range def.aliases {
			kindAliases[strings.ToLower(alias)] = kind
		}
		kindAliases[strings.ToLower(def.name)] = kind
	}
}

// Lookup resolves a user-provided kind string to a supported kind definition.
func Lookup(kind string) (Kind, bool) {
	key := strings.ToLower(strings.TrimSpace(kind))
	if key == "" {
		return Kind{}, false
	}
	result, ok := kindAliases[key]
	return result, ok
}

// DefaultKinds returns the canonical kind names sorted alphabetically.
func DefaultKinds() []string {
	set := make(map[string]struct{})
	for _, def := range kindDefinitions {
		set[def.name] = struct{}{}
	}
	names := make([]string, 0, len(set))
	for name := range set {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
