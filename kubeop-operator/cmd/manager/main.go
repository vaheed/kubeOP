package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	appv1alpha1 "github.com/vaheed/kubeOP/kubeop-operator/api/v1alpha1"
	"github.com/vaheed/kubeOP/kubeop-operator/controllers"
	"github.com/vaheed/kubeOP/kubeop-operator/internal/bootstrap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	ctrlzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

var (
	scheme   = runtime.NewScheme()
	setupLog *zap.SugaredLogger
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(appv1alpha1.AddToScheme(scheme))
}

func main() {
	var metricsAddr string
	var probeAddr string
	var enableLeaderElection bool

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager.")
	flag.Parse()

	logger := buildLogger()
	defer func() {
		_ = logger.Sync()
	}()

	setupLog = logger.Named("setup")
	ctrl.SetLogger(ctrlzap.New(ctrlzap.UseDevMode(true)))

	cfg, err := ctrl.GetConfig()
	if err != nil {
		setupLog.Errorw("Failed to load Kubernetes configuration", "error", err)
		os.Exit(1)
	}

	cfg = applyDefaultQPS(cfg)

	bootstrapLogger := logger.Named("bootstrap")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if err := bootstrap.EnsureAppCRD(ctx, cfg, bootstrapLogger); err != nil {
		setupLog.Errorw("Failed to ensure App CRD", "error", err)
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "kubeop-operator",
	})
	if err != nil {
		setupLog.Errorw("Unable to start manager", "error", err)
		os.Exit(1)
	}

	reconciler := &controllers.AppReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		Logger: logger.Named("controller"),
	}
	if err := reconciler.SetupWithManager(mgr); err != nil {
		setupLog.Errorw("Unable to create controller", "controller", "App", "error", err)
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		setupLog.Errorw("Unable to set up health check", "error", err)
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("ready", healthz.Ping); err != nil {
		setupLog.Errorw("Unable to set up ready check", "error", err)
		os.Exit(1)
	}

	setupLog.Infow("Starting manager", "metrics", metricsAddr, "probes", probeAddr, "leaderElection", enableLeaderElection)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Errorw("Manager exited", "error", err)
		os.Exit(1)
	}
}

func buildLogger() *zap.SugaredLogger {
	encoderConfig := zap.NewDevelopmentEncoderConfig()
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.UTC().Format(time.RFC3339))
	}
	cfg := zap.Config{
		Level:            zap.NewAtomicLevelAt(zap.InfoLevel),
		Development:      true,
		Encoding:         "console",
		EncoderConfig:    encoderConfig,
		OutputPaths:      []string{"stdout"},
		ErrorOutputPaths: []string{"stderr"},
	}
	logger, err := cfg.Build()
	if err != nil {
		panic(fmt.Errorf("build logger: %w", err))
	}
	return logger.Sugar()
}

func applyDefaultQPS(cfg *rest.Config) *rest.Config {
	if cfg.QPS == 0 {
		cfg.QPS = 50
	}
	if cfg.Burst == 0 {
		cfg.Burst = 100
	}
	return cfg
}
