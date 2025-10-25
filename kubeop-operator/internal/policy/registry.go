package policy

import (
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	// ConfigMapName is the expected ConfigMap name providing registry policy configuration.
	ConfigMapName = "registry-policy"

	allowedRegistriesKey   = "allowedRegistries"
	allowedRepositoriesKey = "allowedRepositories"
	requireCosignKey       = "requireCosign"
)

// RegistryPolicy defines registry and repository allowlists for workloads.
// A nil pointer indicates no restrictions.
type RegistryPolicy struct {
	AllowedRegistries   []string
	AllowedRepositories []string
	RequireCosign       bool
}

// ParseConfigMap constructs a RegistryPolicy from a ConfigMap. Missing keys
// yield empty allowlists. When the ConfigMap has no relevant data a nil policy
// is returned.
func ParseConfigMap(cm *corev1.ConfigMap) (*RegistryPolicy, error) {
	if cm == nil {
		return nil, nil
	}
	var policy RegistryPolicy
	if raw, ok := cm.Data[allowedRegistriesKey]; ok {
		policy.AllowedRegistries = parseList(raw)
	}
	if raw, ok := cm.Data[allowedRepositoriesKey]; ok {
		policy.AllowedRepositories = parseList(raw)
	}
	if raw, ok := cm.Data[requireCosignKey]; ok {
		value := strings.TrimSpace(strings.ToLower(raw))
		switch value {
		case "true", "1", "yes", "y":
			policy.RequireCosign = true
		case "false", "0", "no", "n", "":
			policy.RequireCosign = false
		default:
			return nil, fmt.Errorf("invalid boolean value %q for %s", raw, requireCosignKey)
		}
	}
	if policy.IsEmpty() && !policy.RequireCosign {
		return nil, nil
	}
	policy.normalise()
	return &policy, nil
}

// IsEmpty reports whether the policy has allowlist data.
func (p *RegistryPolicy) IsEmpty() bool {
	if p == nil {
		return true
	}
	return len(p.AllowedRegistries) == 0 && len(p.AllowedRepositories) == 0
}

// AllowsRegistry checks if the provided host is within the configured registry allowlist.
// Empty allowlists permit any host.
func (p *RegistryPolicy) AllowsRegistry(host string) bool {
	if p == nil || len(p.AllowedRegistries) == 0 {
		return true
	}
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	for _, allowed := range p.AllowedRegistries {
		if matchesHostPattern(allowed, host) {
			return true
		}
	}
	return false
}

// AllowsRepository checks if the provided repository matches the allowlist.
// Repository values are expected in "host/path" form.
func (p *RegistryPolicy) AllowsRepository(repository string) bool {
	if p == nil || len(p.AllowedRepositories) == 0 {
		return true
	}
	repository = strings.ToLower(strings.TrimSpace(repository))
	if repository == "" {
		return false
	}
	for _, allowed := range p.AllowedRepositories {
		if matchesRepoPattern(allowed, repository) {
			return true
		}
	}
	return false
}

func (p *RegistryPolicy) normalise() {
	if p == nil {
		return
	}
	p.AllowedRegistries = normaliseList(p.AllowedRegistries)
	p.AllowedRepositories = normaliseList(p.AllowedRepositories)
}

func normaliseList(values []string) []string {
	if len(values) == 0 {
		return values
	}
	normalised := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.ToLower(strings.TrimSpace(value))
		if trimmed == "" {
			continue
		}
		normalised = append(normalised, trimmed)
	}
	sort.Strings(normalised)
	return normalised
}

func parseList(raw string) []string {
	var entries []string
	for _, line := range strings.Split(raw, "\n") {
		for _, part := range strings.Split(line, ",") {
			trimmed := strings.TrimSpace(part)
			if trimmed == "" {
				continue
			}
			entries = append(entries, trimmed)
		}
	}
	return entries
}

func matchesHostPattern(pattern, host string) bool {
	pattern = strings.ToLower(pattern)
	if pattern == "" {
		return false
	}
	if strings.HasPrefix(pattern, "*.") {
		suffix := strings.TrimPrefix(pattern, "*.")
		return host == suffix || strings.HasSuffix(host, "."+suffix)
	}
	return host == pattern
}

func matchesRepoPattern(pattern, repo string) bool {
	pattern = strings.ToLower(pattern)
	if pattern == "" {
		return false
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(repo, prefix)
	}
	return repo == pattern
}
