package service

import (
	"strings"

	"kubeop/internal/store"
	"kubeop/internal/util"
)

const (
	labelAppID       = "kubeop.app.id"
	labelAppName     = "kubeop.app.name"
	labelProjectID   = "kubeop.project.id"
	labelProjectName = "kubeop.project.name"
	labelClusterID   = "kubeop.cluster.id"
	labelTenantID    = "kubeop.tenant.id"
)

// CanonicalAppLabels returns the canonical kubeOP tenancy labels that should be
// attached to every managed object for an application deployment. Values are
// trimmed and empty entries are omitted.
func CanonicalAppLabels(project store.Project, appName, kubeName, appID string) map[string]string {
	labels := map[string]string{}
	add := func(key, value string) {
		value = strings.TrimSpace(value)
		if value != "" {
			labels[key] = value
		}
	}
	add(labelAppID, appID)
	if strings.TrimSpace(kubeName) == "" {
		if strings.TrimSpace(appName) != "" {
			kubeName = util.Slugify(appName)
		} else {
			kubeName = ""
		}
	}
	add(labelAppName, kubeName)
	add(labelProjectID, project.ID)
	if strings.TrimSpace(project.Name) != "" {
		add(labelProjectName, util.Slugify(project.Name))
	} else if strings.TrimSpace(project.Namespace) != "" {
		add(labelProjectName, util.Slugify(project.Namespace))
	}
	add(labelClusterID, project.ClusterID)
	add(labelTenantID, project.UserID)
	return labels
}
