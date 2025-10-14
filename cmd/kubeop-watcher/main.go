package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
)

type watcherConfig struct {
	ClusterID      string
	EventsURL      string
	Token          string
	WatchKinds     []string
	LabelSelector  string
	RequiredLabels []string
	BatchMax       int
	BatchWindow    time.Duration
	HTTPTimeout    time.Duration
	StorePath      string
	Heartbeat      time.Duration
	KubeconfigPath string
	ListenAddr     string
}

const (
	defaultLabelSelector  = "kubeop.project.id,kubeop.app.id,kubeop.tenant.id"
	defaultStorePath      = "/var/lib/kubeop-watcher/state.db"
	defaultListenAddr     = ":8081"
	managerInitialBackoff = time.Second
	managerBackoffCeiling = 30 * time.Second
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
	logger.Info("starting kubeop watcher", zap.String("cluster_id", cfg.ClusterID), zap.Strings("kinds", cfg.WatchKinds))

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

	sinkLogger := logger.With(zap.String("component", "sink"))
	eventSink, err := sink.New(sink.Config{
		URL:         cfg.EventsURL,
		Token:       cfg.Token,
		BatchMax:    cfg.BatchMax,
		BatchWindow: cfg.BatchWindow,
		HTTPTimeout: cfg.HTTPTimeout,
		UserAgent:   fmt.Sprintf("kubeop-watcher/%s", version.Version),
	}, sinkLogger)
	if err != nil {
		logger.Fatal("setup sink", zap.Error(err))
	}

	watchKinds := resolveKinds(logger, cfg.WatchKinds)
	if len(watchKinds) == 0 {
		logger.Fatal("no supported watch kinds configured")
	}

	manager, err := watch.NewManager(dynamicClient, store, eventSink, watch.Options{
		Kinds:          watchKinds,
		LabelSelector:  cfg.LabelSelector,
		RequiredLabels: cfg.RequiredLabels,
		ClusterID:      cfg.ClusterID,
	})
	if err != nil {
		logger.Fatal("create watch manager", zap.Error(err))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

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
		Handler: buildMux(manager, eventSink),
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

func loadConfig() (watcherConfig, error) {
	cfg := watcherConfig{}
	cfg.ClusterID = strings.TrimSpace(os.Getenv("CLUSTER_ID"))
	if cfg.ClusterID == "" {
		return cfg, errors.New("CLUSTER_ID is required")
	}
	cfg.EventsURL = strings.TrimSpace(os.Getenv("KUBEOP_EVENTS_URL"))
	if cfg.EventsURL == "" {
		return cfg, errors.New("KUBEOP_EVENTS_URL is required")
	}
	cfg.Token = strings.TrimSpace(os.Getenv("KUBEOP_TOKEN"))
	if cfg.Token == "" {
		return cfg, errors.New("KUBEOP_TOKEN is required")
	}
	cfg.KubeconfigPath = strings.TrimSpace(os.Getenv("KUBECONFIG"))
	cfg.LabelSelector = strings.TrimSpace(os.Getenv("LABEL_SELECTOR"))
	if cfg.LabelSelector == "" {
		cfg.LabelSelector = defaultLabelSelector
	}
	cfg.RequiredLabels = deriveLabelKeys(cfg.LabelSelector)
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

func buildMux(manager *watch.Manager, s *sink.Sink) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		respondJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if !manager.Ready() {
			respondJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "informers"})
			return
		}
		if !s.Ready() {
			respondJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "not_ready", "reason": "sink"})
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

func startHeartbeat(ctx context.Context, s sink.Enqueuer, cfg watcherConfig) {
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
