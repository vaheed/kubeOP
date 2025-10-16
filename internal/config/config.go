package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	// App
	Env      string `yaml:"env"`
	Port     int    `yaml:"port"`
	LogLevel string `yaml:"logLevel"`

	// Public endpoints
	BaseURL           string `yaml:"baseURL"`
	AllowInsecureHTTP bool   `yaml:"allowInsecureHTTP"`

	// Security
	AdminJWTSecret    string `yaml:"adminJWTSecret"`
	DisableAuth       bool   `yaml:"disableAuth"`
	KcfgEncryptionKey string `yaml:"kcfgEncryptionKey"`

	// DB
	DatabaseURL     string `yaml:"databaseURL"`
	EventsDBEnabled bool   `yaml:"eventsDBEnabled"`
	K8SEventsBridge bool   `yaml:"k8sEventsBridge"`

	// Optional config file path (only from env)
	ConfigFile string `yaml:"-"`

	// Tenancy / Isolation Defaults
	PodSecurityLevel           string `yaml:"podSecurityLevel"`
	DNSNamespaceLabelKey       string `yaml:"dnsNamespaceLabelKey"`
	DNSNamespaceLabelValue     string `yaml:"dnsNamespaceLabelValue"`
	DNSPodLabelKey             string `yaml:"dnsPodLabelKey"`
	DNSPodLabelValue           string `yaml:"dnsPodLabelValue"`
	IngressNamespaceLabelKey   string `yaml:"ingressNamespaceLabelKey"`
	IngressNamespaceLabelValue string `yaml:"ingressNamespaceLabelValue"`

	// Namespace quota defaults
	NamespaceQuotaRequestsCPU           string `yaml:"namespaceQuotaRequestsCPU"`
	NamespaceQuotaLimitsCPU             string `yaml:"namespaceQuotaLimitsCPU"`
	NamespaceQuotaRequestsMemory        string `yaml:"namespaceQuotaRequestsMemory"`
	NamespaceQuotaLimitsMemory          string `yaml:"namespaceQuotaLimitsMemory"`
	NamespaceQuotaRequestsEphemeral     string `yaml:"namespaceQuotaRequestsEphemeral"`
	NamespaceQuotaLimitsEphemeral       string `yaml:"namespaceQuotaLimitsEphemeral"`
	NamespaceQuotaPods                  string `yaml:"namespaceQuotaPods"`
	NamespaceQuotaServices              string `yaml:"namespaceQuotaServices"`
	NamespaceQuotaServicesLoadBalancers string `yaml:"namespaceQuotaServicesLoadBalancers"`
	NamespaceQuotaConfigMaps            string `yaml:"namespaceQuotaConfigMaps"`
	NamespaceQuotaSecrets               string `yaml:"namespaceQuotaSecrets"`
	NamespaceQuotaPVCs                  string `yaml:"namespaceQuotaPVCs"`
	NamespaceQuotaRequestsStorage       string `yaml:"namespaceQuotaRequestsStorage"`
	NamespaceQuotaDeployments           string `yaml:"namespaceQuotaDeployments"`
	NamespaceQuotaReplicaSets           string `yaml:"namespaceQuotaReplicaSets"`
	NamespaceQuotaStatefulSets          string `yaml:"namespaceQuotaStatefulSets"`
	NamespaceQuotaJobs                  string `yaml:"namespaceQuotaJobs"`
	NamespaceQuotaCronJobs              string `yaml:"namespaceQuotaCronJobs"`
	NamespaceQuotaIngresses             string `yaml:"namespaceQuotaIngresses"`
	NamespaceQuotaScopes                string `yaml:"namespaceQuotaScopes"`
	NamespaceQuotaPriorityClasses       string `yaml:"namespaceQuotaPriorityClasses"`

	// Namespace LimitRange defaults (per container)
	NamespaceLRContainerMaxCPU                  string `yaml:"namespaceLRContainerMaxCPU"`
	NamespaceLRContainerMaxMemory               string `yaml:"namespaceLRContainerMaxMemory"`
	NamespaceLRContainerMinCPU                  string `yaml:"namespaceLRContainerMinCPU"`
	NamespaceLRContainerMinMemory               string `yaml:"namespaceLRContainerMinMemory"`
	NamespaceLRContainerDefaultCPU              string `yaml:"namespaceLRContainerDefaultCPU"`
	NamespaceLRContainerDefaultMemory           string `yaml:"namespaceLRContainerDefaultMemory"`
	NamespaceLRContainerDefaultRequestCPU       string `yaml:"namespaceLRContainerDefaultRequestCPU"`
	NamespaceLRContainerDefaultRequestMemory    string `yaml:"namespaceLRContainerDefaultRequestMemory"`
	NamespaceLRContainerMaxEphemeral            string `yaml:"namespaceLRContainerMaxEphemeral"`
	NamespaceLRContainerMinEphemeral            string `yaml:"namespaceLRContainerMinEphemeral"`
	NamespaceLRContainerDefaultEphemeral        string `yaml:"namespaceLRContainerDefaultEphemeral"`
	NamespaceLRContainerDefaultRequestEphemeral string `yaml:"namespaceLRContainerDefaultRequestEphemeral"`
	NamespaceLRExtMax                           string `yaml:"namespaceLRExtMax"`
	NamespaceLRExtMin                           string `yaml:"namespaceLRExtMin"`
	NamespaceLRExtDefault                       string `yaml:"namespaceLRExtDefault"`
	NamespaceLRExtDefaultRequest                string `yaml:"namespaceLRExtDefaultRequest"`

	// Projects placement
	ProjectsInUserNamespace bool `yaml:"projectsInUserNamespace"`

	// Project-level LimitRange defaults (should be equal or lower than namespace defaults)
	ProjectLRRequestCPU    string `yaml:"projectLRRequestCPU"`
	ProjectLRRequestMemory string `yaml:"projectLRRequestMemory"`
	ProjectLRLimitCPU      string `yaml:"projectLRLimitCPU"`
	ProjectLRLimitMemory   string `yaml:"projectLRLimitMemory"`

	// Scheduler
	ClusterHealthIntervalSeconds int `yaml:"clusterHealthIntervalSeconds"`

	// Ingress/LB and PaaS
	PaaSDomain                 string `yaml:"paasDomain"`
	PaaSWildcardEnabled        bool   `yaml:"paasWildcardEnabled"`
	LBDriver                   string `yaml:"lbDriver"`
	LBMetallbPool              string `yaml:"lbMetallbPool"`
	MaxLoadBalancersPerProject int    `yaml:"maxLoadBalancersPerProject"`
	EnableCertManager          bool   `yaml:"enableCertManager"`

	// Webhooks
	GitWebhookSecret string `yaml:"gitWebhookSecret"`

	// Watcher auto-deployment
	WatcherAutoDeploy          bool   `yaml:"watcherAutoDeploy"`
	WatcherAutoDeploySource    string `yaml:"-"`
	WatcherNamespace           string `yaml:"watcherNamespace"`
	WatcherNamespaceCreate     bool   `yaml:"watcherNamespaceCreate"`
	WatcherDeploymentName      string `yaml:"watcherDeploymentName"`
	WatcherServiceAccount      string `yaml:"watcherServiceAccount"`
	WatcherSecretName          string `yaml:"watcherSecretName"`
	WatcherPVCName             string `yaml:"watcherPVCName"`
	WatcherPVCStorageClass     string `yaml:"watcherPVCStorageClass"`
	WatcherPVCSize             string `yaml:"watcherPVCSize"`
	WatcherImage               string `yaml:"watcherImage"`
	WatcherEventsURL           string `yaml:"watcherEventsURL"`
	WatcherToken               string `yaml:"watcherToken"`
	WatcherBatchMax            int    `yaml:"watcherBatchMax"`
	WatcherBatchWindowMillis   int    `yaml:"watcherBatchWindowMillis"`
	WatcherStorePath           string `yaml:"watcherStorePath"`
	WatcherHeartbeatMinutes    int    `yaml:"watcherHeartbeatMinutes"`
	WatcherWaitForReady        bool   `yaml:"watcherWaitForReady"`
	WatcherReadyTimeoutSeconds int    `yaml:"watcherReadyTimeoutSeconds"`

	// External DNS automation (optional)
	ExternalDNSProvider string `yaml:"externalDNSProvider"` // cloudflare|powerdns|""
	ExternalDNSTTL      int    `yaml:"externalDNSTTL"`
	// Cloudflare
	CFAPIToken string `yaml:"cfAPIToken"`
	CFZoneID   string `yaml:"cfZoneID"`
	// PowerDNS
	PDNSAPIURL   string `yaml:"pdnsAPIURL"`
	PDNSAPIKey   string `yaml:"pdnsAPIKey"`
	PDNSServerID string `yaml:"pdnsServerID"`
	PDNSZone     string `yaml:"pdnsZone"` // defaults to PAAS_DOMAIN if empty
}

// Load reads an optional YAML config file and environment variables.
// Precedence: defaults < file < environment variables.
func Load() (*Config, error) {
	cfg := &Config{}
	hadFile := false
	watcherAutoDeployConfigured := false
	watcherNamespaceCreateConfigured := false
	watcherWaitForReadyConfigured := false
	_, watcherAutoDeployEnvSet := os.LookupEnv("WATCHER_AUTO_DEPLOY")
	_, watcherNamespaceCreateEnvSet := os.LookupEnv("WATCHER_NAMESPACE_CREATE")
	_, watcherWaitForReadyEnvSet := os.LookupEnv("WATCHER_WAIT_FOR_READY")

	// 1) Read optional YAML config (path comes from env)
	cfg.ConfigFile = getEnv("CONFIG_FILE", "")
	if cfg.ConfigFile != "" {
		if _, err := os.Stat(cfg.ConfigFile); err == nil {
			by, err := os.ReadFile(cfg.ConfigFile)
			if err != nil {
				return nil, fmt.Errorf("read config file: %w", err)
			}
			if err := yaml.Unmarshal(by, cfg); err != nil {
				return nil, fmt.Errorf("parse config file: %w", err)
			}
			var raw map[string]any
			if err := yaml.Unmarshal(by, &raw); err == nil {
				if _, ok := raw["watcherAutoDeploy"]; ok {
					watcherAutoDeployConfigured = true
				}
				if _, ok := raw["watcherNamespaceCreate"]; ok {
					watcherNamespaceCreateConfigured = true
				}
				if _, ok := raw["watcherWaitForReady"]; ok {
					watcherWaitForReadyConfigured = true
				}
			}
			hadFile = true
		}
	}

	// 2) Apply defaults for any still-zero fields
	if strings.TrimSpace(cfg.Env) == "" {
		cfg.Env = "development"
	}
	if cfg.Port == 0 {
		cfg.Port = 8080
	}
	if strings.TrimSpace(cfg.LogLevel) == "" {
		cfg.LogLevel = "info"
	}
	if strings.TrimSpace(cfg.AdminJWTSecret) == "" {
		cfg.AdminJWTSecret = "dev-admin-secret-change-me"
	}
	if strings.TrimSpace(cfg.KcfgEncryptionKey) == "" {
		cfg.KcfgEncryptionKey = "dev-not-secure-key"
	}
	if strings.TrimSpace(cfg.DatabaseURL) == "" {
		cfg.DatabaseURL = "postgres://postgres:postgres@localhost:5432/kubeop?sslmode=disable"
	}
	if !hadFile {
		cfg.EventsDBEnabled = true
	}

	// Tenancy defaults
	if cfg.PodSecurityLevel == "" {
		cfg.PodSecurityLevel = "baseline"
	}
	if cfg.DNSNamespaceLabelKey == "" {
		cfg.DNSNamespaceLabelKey = "kubernetes.io/metadata.name"
	}
	if cfg.DNSNamespaceLabelValue == "" {
		cfg.DNSNamespaceLabelValue = "kube-system"
	}
	if cfg.DNSPodLabelKey == "" {
		cfg.DNSPodLabelKey = "k8s-app"
	}
	if cfg.DNSPodLabelValue == "" {
		cfg.DNSPodLabelValue = "kube-dns"
	}
	if cfg.IngressNamespaceLabelKey == "" {
		cfg.IngressNamespaceLabelKey = "kubeop.io/ingress"
	}
	if cfg.IngressNamespaceLabelValue == "" {
		cfg.IngressNamespaceLabelValue = "true"
	}
	// Default project placement: shared user namespace (one user, many projects)
	if !hadFile {
		cfg.ProjectsInUserNamespace = true
	}

	// Namespace quota defaults
	if cfg.NamespaceQuotaRequestsCPU == "" {
		cfg.NamespaceQuotaRequestsCPU = "2"
	}
	if cfg.NamespaceQuotaLimitsCPU == "" {
		cfg.NamespaceQuotaLimitsCPU = "4"
	}
	if cfg.NamespaceQuotaRequestsMemory == "" {
		cfg.NamespaceQuotaRequestsMemory = "4Gi"
	}
	if cfg.NamespaceQuotaLimitsMemory == "" {
		cfg.NamespaceQuotaLimitsMemory = "8Gi"
	}
	if cfg.NamespaceQuotaRequestsEphemeral == "" {
		cfg.NamespaceQuotaRequestsEphemeral = "10Gi"
	}
	if cfg.NamespaceQuotaLimitsEphemeral == "" {
		cfg.NamespaceQuotaLimitsEphemeral = "20Gi"
	}
	if cfg.NamespaceQuotaPods == "" {
		cfg.NamespaceQuotaPods = "30"
	}
	if cfg.NamespaceQuotaServices == "" {
		cfg.NamespaceQuotaServices = "10"
	}
	if cfg.NamespaceQuotaServicesLoadBalancers == "" {
		cfg.NamespaceQuotaServicesLoadBalancers = "1"
	}
	if cfg.NamespaceQuotaConfigMaps == "" {
		cfg.NamespaceQuotaConfigMaps = "100"
	}
	if cfg.NamespaceQuotaSecrets == "" {
		cfg.NamespaceQuotaSecrets = "100"
	}
	if cfg.NamespaceQuotaPVCs == "" {
		cfg.NamespaceQuotaPVCs = "10"
	}
	if cfg.NamespaceQuotaRequestsStorage == "" {
		cfg.NamespaceQuotaRequestsStorage = "200Gi"
	}
	if cfg.NamespaceQuotaDeployments == "" {
		cfg.NamespaceQuotaDeployments = "20"
	}
	if cfg.NamespaceQuotaReplicaSets == "" {
		cfg.NamespaceQuotaReplicaSets = "40"
	}
	if cfg.NamespaceQuotaStatefulSets == "" {
		cfg.NamespaceQuotaStatefulSets = "5"
	}
	if cfg.NamespaceQuotaJobs == "" {
		cfg.NamespaceQuotaJobs = "20"
	}
	if cfg.NamespaceQuotaCronJobs == "" {
		cfg.NamespaceQuotaCronJobs = "10"
	}
	if cfg.NamespaceQuotaIngresses == "" {
		cfg.NamespaceQuotaIngresses = "10"
	}
	if cfg.NamespaceQuotaScopes == "" {
		cfg.NamespaceQuotaScopes = "NotBestEffort"
	}
	// Priority classes default empty string (all allowed)

	// Namespace LimitRange defaults
	if cfg.NamespaceLRContainerMaxCPU == "" {
		cfg.NamespaceLRContainerMaxCPU = "2"
	}
	if cfg.NamespaceLRContainerMaxMemory == "" {
		cfg.NamespaceLRContainerMaxMemory = "2Gi"
	}
	if cfg.NamespaceLRContainerMinCPU == "" {
		cfg.NamespaceLRContainerMinCPU = "100m"
	}
	if cfg.NamespaceLRContainerMinMemory == "" {
		cfg.NamespaceLRContainerMinMemory = "128Mi"
	}
	if cfg.NamespaceLRContainerDefaultCPU == "" {
		cfg.NamespaceLRContainerDefaultCPU = "500m"
	}
	if cfg.NamespaceLRContainerDefaultMemory == "" {
		cfg.NamespaceLRContainerDefaultMemory = "512Mi"
	}
	if cfg.NamespaceLRContainerDefaultRequestCPU == "" {
		cfg.NamespaceLRContainerDefaultRequestCPU = "300m"
	}
	if cfg.NamespaceLRContainerDefaultRequestMemory == "" {
		cfg.NamespaceLRContainerDefaultRequestMemory = "256Mi"
	}
	if cfg.NamespaceLRContainerMaxEphemeral == "" {
		cfg.NamespaceLRContainerMaxEphemeral = "2Gi"
	}
	if cfg.NamespaceLRContainerMinEphemeral == "" {
		cfg.NamespaceLRContainerMinEphemeral = "128Mi"
	}
	if cfg.NamespaceLRContainerDefaultEphemeral == "" {
		cfg.NamespaceLRContainerDefaultEphemeral = "512Mi"
	}
	if cfg.NamespaceLRContainerDefaultRequestEphemeral == "" {
		cfg.NamespaceLRContainerDefaultRequestEphemeral = "256Mi"
	}
	// Extended resources default to empty so GPU-capable quotas are opt-in.
	// NamespaceLRExtMin/Default/DefaultRequest remain empty unless configured.

	// Project-level defaults (independent of namespace defaults)
	if cfg.ProjectLRRequestCPU == "" {
		cfg.ProjectLRRequestCPU = "100m"
	}
	if cfg.ProjectLRRequestMemory == "" {
		cfg.ProjectLRRequestMemory = "128Mi"
	}
	if cfg.ProjectLRLimitCPU == "" {
		cfg.ProjectLRLimitCPU = "1"
	}
	if cfg.ProjectLRLimitMemory == "" {
		cfg.ProjectLRLimitMemory = "1Gi"
	}

	// Watcher defaults
	if cfg.WatcherDeploymentName == "" {
		cfg.WatcherDeploymentName = "kubeop-watcher"
	}
	if cfg.WatcherServiceAccount == "" {
		cfg.WatcherServiceAccount = "kubeop-watcher"
	}
	if cfg.WatcherSecretName == "" {
		cfg.WatcherSecretName = "kubeop-watcher"
	}
	if cfg.WatcherNamespace == "" {
		cfg.WatcherNamespace = "kubeop-system"
	}
	if cfg.WatcherImage == "" {
		cfg.WatcherImage = "ghcr.io/vaheed/kubeop:watcher"
	}
	if cfg.WatcherStorePath == "" {
		cfg.WatcherStorePath = "/var/lib/kubeop-watcher/state.db"
	}
	if !hadFile {
		cfg.WatcherWaitForReady = true
	}
	if cfg.WatcherReadyTimeoutSeconds <= 0 {
		cfg.WatcherReadyTimeoutSeconds = 180
	}

	// 3) Override from environment
	cfg.Env = getEnv("APP_ENV", cfg.Env)
	cfg.Port = getEnvInt("PORT", cfg.Port)
	cfg.LogLevel = getEnv("LOG_LEVEL", cfg.LogLevel)
	cfg.AdminJWTSecret = getEnv("ADMIN_JWT_SECRET", cfg.AdminJWTSecret)
	cfg.DisableAuth = getEnvBool("DISABLE_AUTH", cfg.DisableAuth)
	cfg.KcfgEncryptionKey = getEnv("KCFG_ENCRYPTION_KEY", cfg.KcfgEncryptionKey)
	cfg.DatabaseURL = getEnv("DATABASE_URL", cfg.DatabaseURL)
	cfg.EventsDBEnabled = getEnvBool("EVENTS_DB_ENABLED", cfg.EventsDBEnabled)
	cfg.K8SEventsBridge = getEnvBool("K8S_EVENTS_BRIDGE", cfg.K8SEventsBridge)

	cfg.PodSecurityLevel = getEnv("POD_SECURITY_LEVEL", cfg.PodSecurityLevel)
	cfg.DNSNamespaceLabelKey = getEnv("DNS_NS_LABEL_KEY", cfg.DNSNamespaceLabelKey)
	cfg.DNSNamespaceLabelValue = getEnv("DNS_NS_LABEL_VALUE", cfg.DNSNamespaceLabelValue)
	cfg.DNSPodLabelKey = getEnv("DNS_POD_LABEL_KEY", cfg.DNSPodLabelKey)
	cfg.DNSPodLabelValue = getEnv("DNS_POD_LABEL_VALUE", cfg.DNSPodLabelValue)
	cfg.IngressNamespaceLabelKey = getEnv("INGRESS_NS_LABEL_KEY", cfg.IngressNamespaceLabelKey)
	cfg.IngressNamespaceLabelValue = getEnv("INGRESS_NS_LABEL_VALUE", cfg.IngressNamespaceLabelValue)

	cfg.NamespaceQuotaRequestsCPU = getEnv("KUBEOP_DEFAULT_REQUESTS_CPU", cfg.NamespaceQuotaRequestsCPU)
	cfg.NamespaceQuotaLimitsCPU = getEnv("KUBEOP_DEFAULT_LIMITS_CPU", cfg.NamespaceQuotaLimitsCPU)
	cfg.NamespaceQuotaRequestsMemory = getEnv("KUBEOP_DEFAULT_REQUESTS_MEMORY", cfg.NamespaceQuotaRequestsMemory)
	cfg.NamespaceQuotaLimitsMemory = getEnv("KUBEOP_DEFAULT_LIMITS_MEMORY", cfg.NamespaceQuotaLimitsMemory)
	cfg.NamespaceQuotaRequestsEphemeral = getEnv("KUBEOP_DEFAULT_REQUESTS_EPHEMERAL", cfg.NamespaceQuotaRequestsEphemeral)
	cfg.NamespaceQuotaLimitsEphemeral = getEnv("KUBEOP_DEFAULT_LIMITS_EPHEMERAL", cfg.NamespaceQuotaLimitsEphemeral)
	cfg.NamespaceQuotaPods = getEnv("KUBEOP_DEFAULT_PODS", cfg.NamespaceQuotaPods)
	cfg.NamespaceQuotaServices = getEnv("KUBEOP_DEFAULT_SERVICES", cfg.NamespaceQuotaServices)
	cfg.NamespaceQuotaServicesLoadBalancers = getEnv("KUBEOP_DEFAULT_SERVICES_LOADBALANCERS", cfg.NamespaceQuotaServicesLoadBalancers)
	cfg.NamespaceQuotaConfigMaps = getEnv("KUBEOP_DEFAULT_CONFIGMAPS", cfg.NamespaceQuotaConfigMaps)
	cfg.NamespaceQuotaSecrets = getEnv("KUBEOP_DEFAULT_SECRETS", cfg.NamespaceQuotaSecrets)
	cfg.NamespaceQuotaPVCs = getEnv("KUBEOP_DEFAULT_PVCS", cfg.NamespaceQuotaPVCs)
	cfg.NamespaceQuotaRequestsStorage = getEnv("KUBEOP_DEFAULT_REQUESTS_STORAGE", cfg.NamespaceQuotaRequestsStorage)
	cfg.NamespaceQuotaDeployments = getEnv("KUBEOP_DEFAULT_DEPLOYMENTS_APPS", cfg.NamespaceQuotaDeployments)
	cfg.NamespaceQuotaReplicaSets = getEnv("KUBEOP_DEFAULT_REPLICASETS_APPS", cfg.NamespaceQuotaReplicaSets)
	cfg.NamespaceQuotaStatefulSets = getEnv("KUBEOP_DEFAULT_STATEFULSETS_APPS", cfg.NamespaceQuotaStatefulSets)
	cfg.NamespaceQuotaJobs = getEnv("KUBEOP_DEFAULT_JOBS_BATCH", cfg.NamespaceQuotaJobs)
	cfg.NamespaceQuotaCronJobs = getEnv("KUBEOP_DEFAULT_CRONJOBS_BATCH", cfg.NamespaceQuotaCronJobs)
	cfg.NamespaceQuotaIngresses = getEnv("KUBEOP_DEFAULT_INGRESSES_NETWORKING_K8S_IO", cfg.NamespaceQuotaIngresses)
	cfg.NamespaceQuotaScopes = getEnv("KUBEOP_DEFAULT_SCOPES", cfg.NamespaceQuotaScopes)
	cfg.NamespaceQuotaPriorityClasses = getEnv("KUBEOP_DEFAULT_PRIORITY_CLASSES", cfg.NamespaceQuotaPriorityClasses)

	cfg.NamespaceLRContainerMaxCPU = getEnv("KUBEOP_DEFAULT_LR_CONTAINER_MAX_CPU", cfg.NamespaceLRContainerMaxCPU)
	cfg.NamespaceLRContainerMaxMemory = getEnv("KUBEOP_DEFAULT_LR_CONTAINER_MAX_MEMORY", cfg.NamespaceLRContainerMaxMemory)
	cfg.NamespaceLRContainerMinCPU = getEnv("KUBEOP_DEFAULT_LR_CONTAINER_MIN_CPU", cfg.NamespaceLRContainerMinCPU)
	cfg.NamespaceLRContainerMinMemory = getEnv("KUBEOP_DEFAULT_LR_CONTAINER_MIN_MEMORY", cfg.NamespaceLRContainerMinMemory)
	cfg.NamespaceLRContainerDefaultCPU = getEnv("KUBEOP_DEFAULT_LR_CONTAINER_DEFAULT_CPU", cfg.NamespaceLRContainerDefaultCPU)
	cfg.NamespaceLRContainerDefaultMemory = getEnv("KUBEOP_DEFAULT_LR_CONTAINER_DEFAULT_MEMORY", cfg.NamespaceLRContainerDefaultMemory)
	cfg.NamespaceLRContainerDefaultRequestCPU = getEnv("KUBEOP_DEFAULT_LR_CONTAINER_DEFAULTREQUEST_CPU", cfg.NamespaceLRContainerDefaultRequestCPU)
	cfg.NamespaceLRContainerDefaultRequestMemory = getEnv("KUBEOP_DEFAULT_LR_CONTAINER_DEFAULTREQUEST_MEMORY", cfg.NamespaceLRContainerDefaultRequestMemory)
	cfg.NamespaceLRContainerMaxEphemeral = getEnv("KUBEOP_DEFAULT_LR_CONTAINER_MAX_EPHEMERAL", cfg.NamespaceLRContainerMaxEphemeral)
	cfg.NamespaceLRContainerMinEphemeral = getEnv("KUBEOP_DEFAULT_LR_CONTAINER_MIN_EPHEMERAL", cfg.NamespaceLRContainerMinEphemeral)
	cfg.NamespaceLRContainerDefaultEphemeral = getEnv("KUBEOP_DEFAULT_LR_CONTAINER_DEFAULT_EPHEMERAL", cfg.NamespaceLRContainerDefaultEphemeral)
	cfg.NamespaceLRContainerDefaultRequestEphemeral = getEnv("KUBEOP_DEFAULT_LR_CONTAINER_DEFAULTREQUEST_EPHEMERAL", cfg.NamespaceLRContainerDefaultRequestEphemeral)
	cfg.NamespaceLRExtMax = getEnv("KUBEOP_DEFAULT_LR_EXT_MAX", cfg.NamespaceLRExtMax)
	cfg.NamespaceLRExtMin = getEnv("KUBEOP_DEFAULT_LR_EXT_MIN", cfg.NamespaceLRExtMin)
	cfg.NamespaceLRExtDefault = getEnv("KUBEOP_DEFAULT_LR_EXT_DEFAULT", cfg.NamespaceLRExtDefault)
	cfg.NamespaceLRExtDefaultRequest = getEnv("KUBEOP_DEFAULT_LR_EXT_DEFAULTREQUEST", cfg.NamespaceLRExtDefaultRequest)

	cfg.ProjectsInUserNamespace = getEnvBool("PROJECTS_IN_USER_NAMESPACE", cfg.ProjectsInUserNamespace)

	cfg.ProjectLRRequestCPU = getEnv("PROJECT_LR_REQUEST_CPU", cfg.ProjectLRRequestCPU)
	cfg.ProjectLRRequestMemory = getEnv("PROJECT_LR_REQUEST_MEMORY", cfg.ProjectLRRequestMemory)
	cfg.ProjectLRLimitCPU = getEnv("PROJECT_LR_LIMIT_CPU", cfg.ProjectLRLimitCPU)
	cfg.ProjectLRLimitMemory = getEnv("PROJECT_LR_LIMIT_MEMORY", cfg.ProjectLRLimitMemory)

	cfg.ClusterHealthIntervalSeconds = getEnvInt("CLUSTER_HEALTH_INTERVAL_SECONDS", cfg.ClusterHealthIntervalSeconds)

	cfg.BaseURL = getEnv("KUBEOP_BASE_URL", cfg.BaseURL)
	cfg.BaseURL = strings.TrimSuffix(strings.TrimSpace(cfg.BaseURL), "/")
	cfg.AllowInsecureHTTP = getEnvBool("ALLOW_INSECURE_HTTP", cfg.AllowInsecureHTTP)

	cfg.WatcherAutoDeploy = getEnvBool("WATCHER_AUTO_DEPLOY", cfg.WatcherAutoDeploy)
	if watcherAutoDeployEnvSet {
		cfg.WatcherAutoDeploySource = "env"
	} else if watcherAutoDeployConfigured {
		cfg.WatcherAutoDeploySource = "config"
	}
	cfg.WatcherNamespace = getEnv("WATCHER_NAMESPACE", cfg.WatcherNamespace)
	cfg.WatcherNamespaceCreate = getEnvBool("WATCHER_NAMESPACE_CREATE", cfg.WatcherNamespaceCreate)
	cfg.WatcherDeploymentName = getEnv("WATCHER_DEPLOYMENT_NAME", cfg.WatcherDeploymentName)
	cfg.WatcherServiceAccount = getEnv("WATCHER_SERVICE_ACCOUNT", cfg.WatcherServiceAccount)
	cfg.WatcherSecretName = getEnv("WATCHER_SECRET_NAME", cfg.WatcherSecretName)
	cfg.WatcherPVCName = getEnv("WATCHER_PVC_NAME", cfg.WatcherPVCName)
	cfg.WatcherPVCStorageClass = getEnv("WATCHER_PVC_STORAGE_CLASS", cfg.WatcherPVCStorageClass)
	cfg.WatcherPVCSize = getEnv("WATCHER_PVC_SIZE", cfg.WatcherPVCSize)
	cfg.WatcherImage = getEnv("WATCHER_IMAGE", cfg.WatcherImage)
	cfg.WatcherEventsURL = getEnv("WATCHER_EVENTS_URL", cfg.WatcherEventsURL)
	cfg.WatcherToken = getEnv("WATCHER_TOKEN", cfg.WatcherToken)
	cfg.WatcherBatchMax = getEnvInt("WATCHER_BATCH_MAX", cfg.WatcherBatchMax)
	cfg.WatcherBatchWindowMillis = getEnvInt("WATCHER_BATCH_WINDOW_MS", cfg.WatcherBatchWindowMillis)
	cfg.WatcherStorePath = getEnv("WATCHER_STORE_PATH", cfg.WatcherStorePath)
	cfg.WatcherHeartbeatMinutes = getEnvInt("WATCHER_HEARTBEAT_MINUTES", cfg.WatcherHeartbeatMinutes)
	cfg.WatcherWaitForReady = getEnvBool("WATCHER_WAIT_FOR_READY", cfg.WatcherWaitForReady)
	cfg.WatcherReadyTimeoutSeconds = getEnvInt("WATCHER_READY_TIMEOUT_SECONDS", cfg.WatcherReadyTimeoutSeconds)

	// Ingress/LB and PaaS
	if cfg.LBDriver == "" {
		cfg.LBDriver = "metallb"
	}
	cfg.PaaSDomain = getEnv("PAAS_DOMAIN", cfg.PaaSDomain)
	cfg.PaaSWildcardEnabled = getEnvBool("PAAS_WILDCARD_ENABLED", cfg.PaaSWildcardEnabled)
	cfg.EnableCertManager = getEnvBool("ENABLE_CERT_MANAGER", cfg.EnableCertManager)
	cfg.LBDriver = getEnv("LB_DRIVER", cfg.LBDriver)
	cfg.LBMetallbPool = getEnv("LB_METALLB_POOL", cfg.LBMetallbPool)
	if cfg.MaxLoadBalancersPerProject == 0 {
		cfg.MaxLoadBalancersPerProject = 1
	}
	cfg.MaxLoadBalancersPerProject = getEnvInt("MAX_LOADBALANCERS_PER_PROJECT", cfg.MaxLoadBalancersPerProject)

	// Webhooks
	cfg.GitWebhookSecret = getEnv("GIT_WEBHOOK_SECRET", cfg.GitWebhookSecret)

	// External DNS
	cfg.ExternalDNSProvider = getEnv("EXTERNAL_DNS_PROVIDER", cfg.ExternalDNSProvider)
	cfg.ExternalDNSTTL = getEnvInt("EXTERNAL_DNS_TTL", cfg.ExternalDNSTTL)
	if cfg.ExternalDNSTTL <= 0 {
		cfg.ExternalDNSTTL = 300
	}
	cfg.CFAPIToken = getEnv("CF_API_TOKEN", cfg.CFAPIToken)
	cfg.CFZoneID = getEnv("CF_ZONE_ID", cfg.CFZoneID)
	cfg.PDNSAPIURL = getEnv("PDNS_API_URL", cfg.PDNSAPIURL)
	cfg.PDNSAPIKey = getEnv("PDNS_API_KEY", cfg.PDNSAPIKey)
	cfg.PDNSServerID = getEnv("PDNS_SERVER_ID", cfg.PDNSServerID)
	cfg.PDNSZone = getEnv("PDNS_ZONE", cfg.PDNSZone)

	// 4) Validation
	if strings.TrimSpace(cfg.AdminJWTSecret) == "" && !cfg.DisableAuth {
		return nil, errors.New("ADMIN_JWT_SECRET is required unless DISABLE_AUTH=true")
	}
	if strings.TrimSpace(cfg.KcfgEncryptionKey) == "" {
		return nil, errors.New("KCFG_ENCRYPTION_KEY is required")
	}

	if cfg.BaseURL != "" {
		parsed, err := url.Parse(cfg.BaseURL)
		if err != nil {
			return nil, fmt.Errorf("invalid KUBEOP_BASE_URL: %w", err)
		}
		scheme := strings.ToLower(parsed.Scheme)
		if scheme != "https" {
			if !(cfg.AllowInsecureHTTP && scheme == "http") {
				return nil, errors.New("KUBEOP_BASE_URL must use https (set ALLOW_INSECURE_HTTP=true to permit http)")
			}
		}
		cfg.BaseURL = strings.TrimSuffix(parsed.String(), "/")
	}

	if cfg.WatcherNamespace == "" {
		cfg.WatcherNamespace = "kubeop-system"
	}
	if cfg.WatcherDeploymentName == "" {
		cfg.WatcherDeploymentName = "kubeop-watcher"
	}
	if cfg.WatcherServiceAccount == "" {
		cfg.WatcherServiceAccount = "kubeop-watcher"
	}
	if cfg.WatcherSecretName == "" {
		cfg.WatcherSecretName = "kubeop-watcher"
	}
	if cfg.WatcherPVCName == "" {
		cfg.WatcherPVCName = cfg.WatcherDeploymentName + "-state"
	}
	if cfg.WatcherImage == "" {
		cfg.WatcherImage = "ghcr.io/vaheed/kubeop:watcher"
	}
	if cfg.WatcherStorePath == "" {
		cfg.WatcherStorePath = "/var/lib/kubeop-watcher/state.db"
	}
	if cfg.WatcherBatchMax <= 0 || cfg.WatcherBatchMax > 200 {
		cfg.WatcherBatchMax = 200
	}
	if cfg.WatcherBatchWindowMillis <= 0 {
		cfg.WatcherBatchWindowMillis = 1000
	}
	if cfg.WatcherHeartbeatMinutes < 0 {
		cfg.WatcherHeartbeatMinutes = 0
	}
	if cfg.WatcherReadyTimeoutSeconds <= 0 {
		cfg.WatcherReadyTimeoutSeconds = 180
	}
	if !watcherAutoDeployEnvSet && !watcherAutoDeployConfigured {
		if strings.TrimSpace(cfg.BaseURL) != "" {
			cfg.WatcherAutoDeploy = true
			cfg.WatcherAutoDeploySource = "base-url"
		} else if cfg.WatcherAutoDeploySource == "" {
			cfg.WatcherAutoDeploySource = "default"
		}
	}
	if cfg.WatcherAutoDeploySource == "" {
		cfg.WatcherAutoDeploySource = "default"
	}
	if !watcherNamespaceCreateEnvSet && !watcherNamespaceCreateConfigured {
		cfg.WatcherNamespaceCreate = true
	}
	if !watcherWaitForReadyEnvSet && !watcherWaitForReadyConfigured {
		cfg.WatcherWaitForReady = true
	}

	if cfg.WatcherEventsURL == "" && cfg.BaseURL != "" {
		cfg.WatcherEventsURL = cfg.BaseURL + "/v1/events/ingest"
	}

	if cfg.WatcherAutoDeploy {
		if strings.TrimSpace(cfg.WatcherEventsURL) == "" {
			return nil, errors.New("WATCHER_EVENTS_URL is required when WATCHER_AUTO_DEPLOY=true")
		}
		if !cfg.AllowInsecureHTTP && !strings.HasPrefix(strings.ToLower(cfg.WatcherEventsURL), "https://") {
			return nil, errors.New("WATCHER_EVENTS_URL must be https when WATCHER_AUTO_DEPLOY=true")
		}
		if cfg.AllowInsecureHTTP {
			lower := strings.ToLower(cfg.WatcherEventsURL)
			if !(strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://")) {
				return nil, errors.New("WATCHER_EVENTS_URL must be http(s) when WATCHER_AUTO_DEPLOY=true")
			}
		}
		if strings.TrimSpace(cfg.WatcherNamespace) == "" {
			return nil, errors.New("WATCHER_NAMESPACE is required when WATCHER_AUTO_DEPLOY=true")
		}
		if cfg.WatcherWaitForReady && cfg.WatcherReadyTimeoutSeconds <= 0 {
			return nil, errors.New("WATCHER_READY_TIMEOUT_SECONDS must be >0 when WATCHER_WAIT_FOR_READY=true")
		}
	}

	return cfg, nil
}

// WatcherAutoDeployExplanation summarises why watcher auto-deploy is enabled or disabled.
func (c *Config) WatcherAutoDeployExplanation() string {
	if c == nil {
		return "configuration unavailable"
	}
	source := c.WatcherAutoDeploySource
	if source == "" {
		source = "default"
	}
	switch source {
	case "env":
		if c.WatcherAutoDeploy {
			return "enabled via WATCHER_AUTO_DEPLOY environment variable"
		}
		return "disabled via WATCHER_AUTO_DEPLOY environment variable"
	case "config":
		if c.WatcherAutoDeploy {
			if strings.TrimSpace(c.ConfigFile) != "" {
				return fmt.Sprintf("enabled by watcherAutoDeploy in %s", c.ConfigFile)
			}
			return "enabled by configuration file"
		}
		if strings.TrimSpace(c.ConfigFile) != "" {
			return fmt.Sprintf("disabled by watcherAutoDeploy in %s", c.ConfigFile)
		}
		return "disabled by configuration file"
	case "base-url":
		if c.WatcherAutoDeploy {
			return "enabled automatically because KUBEOP_BASE_URL/baseURL is configured"
		}
		return "KUBEOP_BASE_URL configured but auto deploy disabled"
	case "default":
		if c.WatcherAutoDeploy {
			return "enabled by default"
		}
		if strings.TrimSpace(c.BaseURL) == "" {
			return "disabled until KUBEOP_BASE_URL is configured"
		}
		return "disabled by default"
	default:
		if c.WatcherAutoDeploy {
			return "enabled"
		}
		if strings.TrimSpace(c.BaseURL) == "" {
			return "disabled until KUBEOP_BASE_URL is configured"
		}
		return "disabled by configuration"
	}
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return def
}

func getEnvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return def
}

func getEnvBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		switch strings.ToLower(v) {
		case "1", "true", "yes", "y":
			return true
		case "0", "false", "no", "n":
			return false
		}
	}
	return def
}
