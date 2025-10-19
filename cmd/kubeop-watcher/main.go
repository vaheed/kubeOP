package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"kubeop/internal/logging"
	"kubeop/internal/sink"
	"kubeop/internal/state"
	"kubeop/internal/version"
	"kubeop/internal/watch"
	"kubeop/internal/watcher/authmanager"
	"kubeop/internal/watcher/readiness"
)

type WatcherConfig struct {
	ClusterID         string
	BaseURL           string
	EventsURL         string
	HandshakeURL      string
	RegisterURL       string
	RefreshURL        string
	BootstrapToken    string
	WatchKinds        []string
	LabelSelector     string
	RequiredLabels    []string
	NamespacePrefixes []string
	BatchMax          int
	BatchWindow       time.Duration
	HTTPTimeout       time.Duration
	StorePath         string
	Heartbeat         time.Duration
	KubeconfigPath    string
	ListenAddr        string
	AllowInsecure     bool
}

const (
	defaultLabelSelector     = ""
	defaultNamespacePrefixes = "user-"
	defaultStorePath         = "/var/lib/kubeop-watcher/state.db"
	defaultListenAddr        = ":8081"
	managerInitialBackoff    = time.Second
	managerBackoffCeiling    = 30 * time.Second
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}
	logManager, err := logging.Setup(logging.Metadata{ClusterID: cfg.ClusterID, Version: version.Version})
	if err != nil {
		fmt.Fprintf(os.Stderr, "setup logging: %v\n", err)
		os.Exit(1)
	}
	defer logManager.Sync()
	logger := logging.L()
	status := readiness.New()
	logger.Info(
		"starting kubeop watcher",
		zap.String("cluster_id", cfg.ClusterID),
		zap.Strings("kinds", cfg.WatchKinds),
		zap.String("base_url", cfg.BaseURL),
		zap.Bool("allow_insecure_http", cfg.AllowInsecure),
	)

	restCfg, err := buildRESTConfig(cfg.KubeconfigPath)
	if err != nil {
		logger.Fatal("build kubernetes config", zap.Error(err))
	}
	restCfg.UserAgent = fmt.Sprintf("kubeop-watcher/%s", version.Version)
	restCfg.Timeout = 30 * time.Second

	dynamicClient, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		logger.Fatal("create dynamic client", zap.Error(err))
	}

	store, err := state.Open(cfg.StorePath)
	if err != nil {
		logger.Fatal("open state store", zap.Error(err))
	}
	defer store.Close()
	status.MarkStoreReady()

	queueLogger := logger.With(zap.String("component", "event_queue"))
	queue := NewEventQueue(store, queueLogger)

	authLogger := logger.With(zap.String("component", "auth"))
	authMgr := authmanager.New(authmanager.Config{
		ClusterID:      cfg.ClusterID,
		RegisterURL:    cfg.RegisterURL,
		RefreshURL:     cfg.RefreshURL,
		BootstrapToken: cfg.BootstrapToken,
	}, store, nil, authLogger)

	sinkLogger := logger.With(zap.String("component", "sink"))
	eventSink, err := sink.New(sink.Config{
		URL:             cfg.EventsURL,
		BatchMax:        cfg.BatchMax,
		BatchWindow:     cfg.BatchWindow,
		HTTPTimeout:     cfg.HTTPTimeout,
		UserAgent:       fmt.Sprintf("kubeop-watcher/%s", version.Version),
		PersistentQueue: queue,
		AllowInsecure:   cfg.AllowInsecure,
		TokenProvider:   authMgr.AccessToken,
		OnUnauthorized: func(cbCtx context.Context) error {
			refreshCtx := cbCtx
			if refreshCtx == nil {
				refreshCtx = context.Background()
			}
			refreshCtx, cancel := context.WithTimeout(refreshCtx, 15*time.Second)
			defer cancel()
			if err := authMgr.ForceRefreshAfterUnauthorized(refreshCtx); err != nil {
				sinkLogger.Warn("forced token refresh failed", zap.Error(err))
				return err
			}
			return nil
		},
	}, sinkLogger)
	if err != nil {
		logger.Fatal("setup sink", zap.Error(err))
	}

	authMgr.AttachSink(eventSink)

	watchKinds := resolveKinds(logger, cfg.WatchKinds)
	if len(watchKinds) == 0 {
		logger.Fatal("no supported watch kinds configured")
	}

	manager, err := watch.NewManager(dynamicClient, store, eventSink, watch.Options{
		Kinds:             watchKinds,
		LabelSelector:     cfg.LabelSelector,
		RequiredLabels:    cfg.RequiredLabels,
		NamespacePrefixes: cfg.NamespacePrefixes,
		ClusterID:         cfg.ClusterID,
	})
	if err != nil {
		logger.Fatal("create watch manager", zap.Error(err))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := authMgr.Initialize(ctx); err != nil {
		logger.Fatal("initialise auth", zap.Error(err))
	}

	handshakeLogger := logger.With(zap.String("component", "handshake"))
	StartHandshakeLoop(ctx, cfg, status, queue, eventSink, authMgr, handshakeLogger)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		eventSink.Run(ctx)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		runManagerLoop(ctx, manager, logger.With(zap.String("component", "manager")))
	}()

	if cfg.Heartbeat > 0 {
		startHeartbeat(ctx, eventSink, cfg)
	}

	server := &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: buildMux(manager, status),
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		logger.Info("serving probes", zap.String("addr", cfg.ListenAddr))
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("http server error", zap.Error(err))
			cancel()
		}
	}()

	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	_ = server.Shutdown(shutdownCtx)
	eventSink.Stop()
	wg.Wait()
	logger.Info("watcher shutdown complete")
}

func loadConfig() (WatcherConfig, error) {
	cfg := WatcherConfig{}
	cfg.AllowInsecure = parseBool(os.Getenv("ALLOW_INSECURE_HTTP"), false)

	baseRaw := strings.TrimSpace(os.Getenv("KUBEOP_BASE_URL"))
	eventsRaw := strings.TrimSpace(os.Getenv("WATCHER_EVENTS_URL"))
	if eventsRaw == "" {
		eventsRaw = strings.TrimSpace(os.Getenv("KUBEOP_EVENTS_URL"))
	}

	var baseURL *url.URL
	if baseRaw != "" {
		parsed, err := url.Parse(strings.TrimSuffix(baseRaw, "/"))
		if err != nil {
			return cfg, fmt.Errorf("invalid KUBEOP_BASE_URL: %w", err)
		}
		if parsed.Scheme == "" || parsed.Host == "" {
			return cfg, errors.New("KUBEOP_BASE_URL must include scheme and host")
		}
		if parsed.Path != "" && parsed.Path != "/" {
			return cfg, errors.New("KUBEOP_BASE_URL must not include a path")
		}
		parsed.Path = ""
		parsed.RawQuery = ""
		parsed.Fragment = ""
		baseURL = parsed
	}

	var eventsURL *url.URL
	if eventsRaw != "" {
		parsed, err := url.Parse(strings.TrimSpace(eventsRaw))
		if err != nil {
			return cfg, fmt.Errorf("invalid WATCHER_EVENTS_URL: %w", err)
		}
		if parsed.Scheme == "" || parsed.Host == "" {
			return cfg, errors.New("WATCHER_EVENTS_URL must include scheme and host")
		}
		if parsed.Path == "" || parsed.Path == "/" {
			parsed.Path = "/v1/events/ingest"
		}
		if !strings.HasSuffix(parsed.Path, "/v1/events/ingest") {
			return cfg, errors.New("WATCHER_EVENTS_URL must target /v1/events/ingest")
		}
		parsed.RawQuery = ""
		parsed.Fragment = ""
		eventsURL = parsed
	}

	if baseURL == nil && eventsURL == nil {
		return cfg, errors.New("set KUBEOP_BASE_URL or WATCHER_EVENTS_URL so the watcher can reach the API")
	}

	if baseURL == nil {
		baseURL = &url.URL{Scheme: eventsURL.Scheme, Host: eventsURL.Host}
	}
	if eventsURL == nil {
		eventsURL = &url.URL{Scheme: baseURL.Scheme, Host: baseURL.Host, Path: "/v1/events/ingest"}
	}

	if !strings.EqualFold(baseURL.Host, eventsURL.Host) {
		return cfg, fmt.Errorf("watcher base and events hosts differ (base=%s events=%s)", baseURL.Host, eventsURL.Host)
	}

	baseScheme := strings.ToLower(baseURL.Scheme)
	eventsScheme := strings.ToLower(eventsURL.Scheme)
	allowHTTP := cfg.AllowInsecure
	if baseScheme != "https" {
		if !(allowHTTP && baseScheme == "http") {
			return cfg, errors.New("KUBEOP_BASE_URL must use https unless ALLOW_INSECURE_HTTP=true")
		}
	}
	if eventsScheme != "https" {
		if !(allowHTTP && eventsScheme == "http") {
			return cfg, errors.New("WATCHER_EVENTS_URL must use https unless ALLOW_INSECURE_HTTP=true")
		}
	}

	canonicalBase := &url.URL{Scheme: baseScheme, Host: baseURL.Host}
	if allowHTTP && baseScheme == "http" {
		canonicalBase.Scheme = "http"
	} else {
		canonicalBase.Scheme = "https"
	}
	cfg.BaseURL = strings.TrimSuffix(canonicalBase.String(), "/")

	eventsURL.Scheme = canonicalBase.Scheme
	eventsURL.Host = canonicalBase.Host
	eventsURL.Path = "/v1/events/ingest"
	cfg.EventsURL = eventsURL.String()
	cfg.HandshakeURL = cfg.BaseURL + "/v1/watchers/handshake"
	cfg.RegisterURL = cfg.BaseURL + "/v1/watchers/register"
	cfg.RefreshURL = cfg.BaseURL + "/v1/watchers/refresh"
	bootstrap := strings.TrimSpace(os.Getenv("KUBEOP_BOOTSTRAP_TOKEN"))
	if bootstrap == "" {
		bootstrap = strings.TrimSpace(os.Getenv("KUBEOP_TOKEN"))
	}
	cfg.BootstrapToken = bootstrap
	cfg.ClusterID = strings.TrimSpace(os.Getenv("CLUSTER_ID"))
	if cfg.ClusterID == "" {
		return cfg, errors.New("CLUSTER_ID is required (this container runs the watcher agent; use the :latest tag for the API)")
	}
	cfg.KubeconfigPath = strings.TrimSpace(os.Getenv("KUBECONFIG"))
	cfg.LabelSelector = strings.TrimSpace(os.Getenv("LABEL_SELECTOR"))
	if cfg.LabelSelector == "" {
		cfg.LabelSelector = defaultLabelSelector
	}
	cfg.RequiredLabels = deriveLabelKeys(cfg.LabelSelector)
	cfg.NamespacePrefixes = parseList(os.Getenv("WATCH_NAMESPACE_PREFIXES"), []string{defaultNamespacePrefixes})
	cfg.WatchKinds = parseList(os.Getenv("WATCH_KINDS"), watch.DefaultKinds())
	cfg.BatchMax = parseInt(os.Getenv("BATCH_MAX"), 200)
	if cfg.BatchMax <= 0 {
		cfg.BatchMax = 200
	}
	if cfg.BatchMax > 200 {
		cfg.BatchMax = 200
	}
	windowMS := parseInt(os.Getenv("BATCH_WINDOW_MS"), 1000)
	if windowMS <= 0 {
		windowMS = 1000
	}
	cfg.BatchWindow = time.Duration(windowMS) * time.Millisecond
	httpTimeout := parseInt(os.Getenv("HTTP_TIMEOUT_SECONDS"), 15)
	if httpTimeout <= 0 {
		httpTimeout = 15
	}
	cfg.HTTPTimeout = time.Duration(httpTimeout) * time.Second
	cfg.StorePath = strings.TrimSpace(os.Getenv("STORE_PATH"))
	if cfg.StorePath == "" {
		cfg.StorePath = defaultStorePath
	}
	heartbeatMin := parseInt(os.Getenv("HEARTBEAT_MINUTES"), 5)
	if heartbeatMin > 0 {
		cfg.Heartbeat = time.Duration(heartbeatMin) * time.Minute
	}
	cfg.ListenAddr = strings.TrimSpace(os.Getenv("WATCHER_LISTEN_ADDR"))
	if cfg.ListenAddr == "" {
		cfg.ListenAddr = defaultListenAddr
	}
	return cfg, nil
}

func parseList(raw string, fallback []string) []string {
	if strings.TrimSpace(raw) == "" {
		return fallback
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		if _, ok := seen[strings.ToLower(p)]; ok {
			continue
		}
		seen[strings.ToLower(p)] = struct{}{}
		result = append(result, p)
	}
	return result
}

func parseInt(raw string, def int) int {
	if strings.TrimSpace(raw) == "" {
		return def
	}
	val, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return def
	}
	return val
}

func parseBool(raw string, def bool) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "y":
		return true
	case "0", "false", "no", "n":
		return false
	case "":
		return def
	default:
		return def
	}
}

func deriveLabelKeys(selector string) []string {
	parts := strings.Split(selector, ",")
	keys := make([]string, 0, len(parts))
	for _, part := range parts {
		p := strings.TrimSpace(part)
		if p == "" {
			continue
		}
		if strings.ContainsAny(p, "=!:><") {
			continue
		}
		keys = append(keys, p)
	}
	return keys
}

func buildRESTConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	return rest.InClusterConfig()
}

func resolveKinds(logger *zap.Logger, names []string) []watch.Kind {
	kinds := make([]watch.Kind, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	for _, name := range names {
		kind, ok := watch.Lookup(name)
		if !ok {
			logger.Warn("ignoring unsupported kind", zap.String("kind", name))
			continue
		}
		if _, exists := seen[kind.Name]; exists {
			continue
		}
		seen[kind.Name] = struct{}{}
		kinds = append(kinds, kind)
	}
	return kinds
}

func buildMux(manager *watch.Manager, status *readiness.Tracker) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if status == nil || !status.StoreReady() {
			respondJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "state_store"})
			return
		}
		if !manager.Ready() {
			respondJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "informers"})
			return
		}
		handshake := status.HandshakeStatus(60 * time.Second)
		delivery := status.DeliveryStatus(60 * time.Second)

		diagnostics := map[string]any{}
		if handshake.Degraded {
			diag := map[string]any{
				"detail": handshake.Detail,
				"fresh":  handshake.Fresh,
				"ready":  handshake.Ready,
				"ever":   handshake.Ever,
			}
			if !handshake.Last.IsZero() {
				diag["last_handshake"] = handshake.Last.UTC().Format(time.RFC3339Nano)
			}
			diagnostics["handshake"] = diag
		}
		if delivery.Degraded {
			diag := map[string]any{
				"detail":  delivery.Detail,
				"healthy": delivery.Healthy,
				"ever":    delivery.Ever,
			}
			if !delivery.Last.IsZero() {
				diag["last_flush"] = delivery.Last.UTC().Format(time.RFC3339Nano)
			}
			diagnostics["delivery"] = diag
		}
		if len(diagnostics) > 0 {
			respondJSON(w, http.StatusOK, map[string]any{
				"status":      "degraded",
				"diagnostics": diagnostics,
			})
			return
		}
		respondJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	mux.Handle("/metrics", promhttp.Handler())
	return mux
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func startHeartbeat(ctx context.Context, s sink.Enqueuer, cfg WatcherConfig) {
	ticker := time.NewTicker(cfg.Heartbeat)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case t := <-ticker.C:
				event := sink.Event{
					ClusterID: cfg.ClusterID,
					EventType: "Modified",
					Kind:      "Watcher",
					Name:      "heartbeat",
					Summary:   "watcher heartbeat",
					DedupKey:  fmt.Sprintf("heartbeat#%d", t.UnixNano()),
				}
				_ = s.Enqueue(event)
			}
		}
	}()
}

func runManagerLoop(ctx context.Context, manager *watch.Manager, logger *zap.Logger) {
	backoff := managerInitialBackoff
	for {
		if ctx.Err() != nil {
			return
		}
		err := manager.Start(ctx)
		if err == nil || errors.Is(err, context.Canceled) {
			return
		}
		logger.Warn("watch manager exited", zap.Error(err), zap.Duration("backoff", backoff))
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > managerBackoffCeiling {
			backoff = managerBackoffCeiling
		}
	}
}
