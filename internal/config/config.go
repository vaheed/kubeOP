package config

import (
	"errors"
	"fmt"
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
	PublicURL string `yaml:"publicURL"`

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

	// Quotas and Limits defaults
	DefaultQuotaLimitsMemory     string `yaml:"defaultQuotaLimitsMemory"`
	DefaultQuotaLimitsCPU        string `yaml:"defaultQuotaLimitsCPU"`
	DefaultQuotaEphemeralStorage string `yaml:"defaultQuotaEphemeralStorage"`
	DefaultQuotaPVCStorage       string `yaml:"defaultQuotaPVCStorage"`
	DefaultQuotaMaxPods          string `yaml:"defaultQuotaMaxPods"`
	DefaultLRRequestCPU          string `yaml:"defaultLRRequestCPU"`
	DefaultLRRequestMemory       string `yaml:"defaultLRRequestMemory"`
	DefaultLRLimitCPU            string `yaml:"defaultLRLimitCPU"`
	DefaultLRLimitMemory         string `yaml:"defaultLRLimitMemory"`

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

	// Quota defaults
	if cfg.DefaultQuotaLimitsMemory == "" {
		cfg.DefaultQuotaLimitsMemory = "64Gi"
	}
	if cfg.DefaultQuotaLimitsCPU == "" {
		cfg.DefaultQuotaLimitsCPU = "128"
	}
	if cfg.DefaultQuotaEphemeralStorage == "" {
		cfg.DefaultQuotaEphemeralStorage = "64Gi"
	}
	if cfg.DefaultQuotaPVCStorage == "" {
		cfg.DefaultQuotaPVCStorage = "500Gi"
	}
	if cfg.DefaultQuotaMaxPods == "" {
		cfg.DefaultQuotaMaxPods = "50"
	}
	if cfg.DefaultLRRequestCPU == "" {
		cfg.DefaultLRRequestCPU = "100m"
	}
	if cfg.DefaultLRRequestMemory == "" {
		cfg.DefaultLRRequestMemory = "128Mi"
	}
	if cfg.DefaultLRLimitCPU == "" {
		cfg.DefaultLRLimitCPU = "1"
	}
	if cfg.DefaultLRLimitMemory == "" {
		cfg.DefaultLRLimitMemory = "1Gi"
	}

	// Project-level defaults (fallback to namespace defaults if empty)
	if cfg.ProjectLRRequestCPU == "" {
		cfg.ProjectLRRequestCPU = cfg.DefaultLRRequestCPU
	}
	if cfg.ProjectLRRequestMemory == "" {
		cfg.ProjectLRRequestMemory = cfg.DefaultLRRequestMemory
	}
	if cfg.ProjectLRLimitCPU == "" {
		cfg.ProjectLRLimitCPU = cfg.DefaultLRLimitCPU
	}
	if cfg.ProjectLRLimitMemory == "" {
		cfg.ProjectLRLimitMemory = cfg.DefaultLRLimitMemory
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

	cfg.DefaultQuotaLimitsMemory = getEnv("DEFAULT_QUOTA_LIMITS_MEMORY", cfg.DefaultQuotaLimitsMemory)
	cfg.DefaultQuotaLimitsCPU = getEnv("DEFAULT_QUOTA_LIMITS_CPU", cfg.DefaultQuotaLimitsCPU)
	cfg.DefaultQuotaEphemeralStorage = getEnv("DEFAULT_QUOTA_EPHEMERAL_STORAGE", cfg.DefaultQuotaEphemeralStorage)
	cfg.DefaultQuotaPVCStorage = getEnv("DEFAULT_QUOTA_PVC_STORAGE", cfg.DefaultQuotaPVCStorage)
	cfg.DefaultQuotaMaxPods = getEnv("DEFAULT_QUOTA_MAX_PODS", cfg.DefaultQuotaMaxPods)
	cfg.DefaultLRRequestCPU = getEnv("DEFAULT_LR_REQUEST_CPU", cfg.DefaultLRRequestCPU)
	cfg.DefaultLRRequestMemory = getEnv("DEFAULT_LR_REQUEST_MEMORY", cfg.DefaultLRRequestMemory)
	cfg.DefaultLRLimitCPU = getEnv("DEFAULT_LR_LIMIT_CPU", cfg.DefaultLRLimitCPU)
	cfg.DefaultLRLimitMemory = getEnv("DEFAULT_LR_LIMIT_MEMORY", cfg.DefaultLRLimitMemory)

	cfg.ProjectsInUserNamespace = getEnvBool("PROJECTS_IN_USER_NAMESPACE", cfg.ProjectsInUserNamespace)

	cfg.ProjectLRRequestCPU = getEnv("PROJECT_LR_REQUEST_CPU", cfg.ProjectLRRequestCPU)
	cfg.ProjectLRRequestMemory = getEnv("PROJECT_LR_REQUEST_MEMORY", cfg.ProjectLRRequestMemory)
	cfg.ProjectLRLimitCPU = getEnv("PROJECT_LR_LIMIT_CPU", cfg.ProjectLRLimitCPU)
	cfg.ProjectLRLimitMemory = getEnv("PROJECT_LR_LIMIT_MEMORY", cfg.ProjectLRLimitMemory)

	cfg.ClusterHealthIntervalSeconds = getEnvInt("CLUSTER_HEALTH_INTERVAL_SECONDS", cfg.ClusterHealthIntervalSeconds)

	if strings.TrimSpace(cfg.PublicURL) == "" {
		cfg.PublicURL = "https://localhost:8443"
	}
	cfg.PublicURL = getEnv("PUBLIC_URL", cfg.PublicURL)
	cfg.PublicURL = strings.TrimSuffix(strings.TrimSpace(cfg.PublicURL), "/")

	cfg.WatcherAutoDeploy = getEnvBool("WATCHER_AUTO_DEPLOY", cfg.WatcherAutoDeploy)
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

	if cfg.PublicURL != "" && !strings.HasPrefix(strings.ToLower(cfg.PublicURL), "https://") {
		return nil, errors.New("PUBLIC_URL must use https")
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
	if !hadFile {
		if _, ok := os.LookupEnv("WATCHER_AUTO_DEPLOY"); !ok {
			cfg.WatcherAutoDeploy = true
		}
		if _, ok := os.LookupEnv("WATCHER_NAMESPACE_CREATE"); !ok {
			cfg.WatcherNamespaceCreate = true
		}
		if _, ok := os.LookupEnv("WATCHER_WAIT_FOR_READY"); !ok {
			cfg.WatcherWaitForReady = true
		}
	}

	if cfg.WatcherEventsURL == "" && cfg.PublicURL != "" {
		cfg.WatcherEventsURL = cfg.PublicURL + "/v1/events/ingest"
	}

	if cfg.WatcherAutoDeploy {
		if strings.TrimSpace(cfg.WatcherEventsURL) == "" {
			return nil, errors.New("WATCHER_EVENTS_URL is required when WATCHER_AUTO_DEPLOY=true")
		}
		if !strings.HasPrefix(strings.ToLower(cfg.WatcherEventsURL), "https://") {
			return nil, errors.New("WATCHER_EVENTS_URL must be https when WATCHER_AUTO_DEPLOY=true")
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
