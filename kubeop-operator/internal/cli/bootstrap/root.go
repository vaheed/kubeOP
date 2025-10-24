package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	appv1alpha1 "github.com/vaheed/kubeOP/kubeop-operator/apis/paas/v1alpha1"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IOStreams controls standard input/output destinations for the CLI.
type IOStreams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer
}

// GlobalOptions captures CLI-wide flags.
type GlobalOptions struct {
	Kubeconfig string
	Context    string
	Yes        bool
	Output     string
	OutDir     string
}

type commandState struct {
	config *rest.Config
	runner *Runner
	logger *zap.SugaredLogger
	output string
	yes    bool
	clean  func() error
}

type contextKey string

const stateKey contextKey = "bootstrap-state"

// NewCommand constructs the bootstrap CLI root command.
func NewCommand(streams IOStreams) *cobra.Command {
	if streams.In == nil {
		streams.In = os.Stdin
	}
	if streams.Out == nil {
		streams.Out = os.Stdout
	}
	if streams.ErrOut == nil {
		streams.ErrOut = os.Stderr
	}

	opts := &GlobalOptions{}
	cmd := &cobra.Command{
		Use:           "kubeop-bootstrap",
		Short:         "Bootstrap kubeOP control plane resources",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetContext(context.Background())
	cmd.PersistentFlags().StringVar(&opts.Kubeconfig, "kubeconfig", "", "Path to the kubeconfig file")
	cmd.PersistentFlags().StringVar(&opts.Context, "context", "", "Kubeconfig context name")
	cmd.PersistentFlags().BoolVar(&opts.Yes, "yes", false, "Confirm the action without prompting")
	cmd.PersistentFlags().StringVar(&opts.Output, "output", OutputTable, "Output format: table or yaml")
	cmd.PersistentFlags().StringVar(&opts.OutDir, "out-dir", defaultOutDir, "Directory for applied manifest copies")

	cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return initialiseState(cmd, opts, streams)
	}
	cmd.PersistentPostRun = func(cmd *cobra.Command, args []string) {
		cleanupState(cmd)
	}

	cmd.AddCommand(
		newInitCommand(streams),
		newDefaultsCommand(streams),
		newTenantCommand(streams),
		newProjectCommand(streams),
		newDomainCommand(streams),
		newRegistryCommand(streams),
	)

	return cmd
}

func initialiseState(cmd *cobra.Command, opts *GlobalOptions, streams IOStreams) error {
	if opts == nil {
		return fmt.Errorf("global options not initialised")
	}
	if existing := getState(cmd); existing != nil {
		return nil
	}
	loggerCfg := zap.NewProductionConfig()
	loggerCfg.OutputPaths = []string{"stderr"}
	loggerCfg.ErrorOutputPaths = []string{"stderr"}
	logger, err := loggerCfg.Build()
	if err != nil {
		return fmt.Errorf("build logger: %w", err)
	}
	sugar := logger.Sugar()

	cfg, err := buildRestConfig(opts.Kubeconfig, opts.Context)
	if err != nil {
		cleanupLogger(logger)
		return err
	}
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		cleanupLogger(logger)
		return fmt.Errorf("register core scheme: %w", err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		cleanupLogger(logger)
		return fmt.Errorf("register paas scheme: %w", err)
	}
	cl, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		cleanupLogger(logger)
		return fmt.Errorf("create kubernetes client: %w", err)
	}
	sink, err := NewJSONEventSink(streams.ErrOut)
	if err != nil {
		cleanupLogger(logger)
		return err
	}
	runner, err := NewRunner(cl, scheme, sugar, sink, opts.OutDir)
	if err != nil {
		cleanupLogger(logger)
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	state := &commandState{
		config: cfg,
		runner: runner,
		logger: sugar,
		output: opts.Output,
		yes:    opts.Yes,
		clean:  logger.Sync,
	}
	cmd.SetContext(context.WithValue(ctx, stateKey, state))
	return nil
}

func cleanupState(cmd *cobra.Command) {
	state := getState(cmd)
	if state == nil {
		return
	}
	if state.clean != nil {
		if err := state.clean(); err != nil && !strings.Contains(err.Error(), "bad file descriptor") {
			fmt.Fprintf(os.Stderr, "failed to flush logs: %v\n", err)
		}
	}
}

func getState(cmd *cobra.Command) *commandState {
	if cmd == nil {
		return nil
	}
	ctx := cmd.Context()
	if ctx == nil {
		return nil
	}
	state, _ := ctx.Value(stateKey).(*commandState)
	return state
}

func buildRestConfig(kubeconfigPath, contextName string) (*rest.Config, error) {
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}
	overrides := &clientcmd.ConfigOverrides{}
	if contextName != "" {
		overrides.CurrentContext = contextName
	}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)
	cfg, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("build kubernetes config: %w", err)
	}
	cfg.UserAgent = appendUserAgent(cfg.UserAgent)
	return cfg, nil
}

func appendUserAgent(existing string) string {
	const ua = "kubeop-bootstrap/0"
	if strings.TrimSpace(existing) == "" {
		return ua
	}
	if strings.Contains(existing, ua) {
		return existing
	}
	return existing + " " + ua
}

func cleanupLogger(logger *zap.Logger) {
	if logger == nil {
		return
	}
	_ = logger.Sync()
}

func requireConfirmation(state *commandState) error {
	if state == nil {
		return fmt.Errorf("command state missing")
	}
	if !state.yes {
		return errors.New("operation requires --yes to continue")
	}
	return nil
}
