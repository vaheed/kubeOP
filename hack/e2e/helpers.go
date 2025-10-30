package e2e

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type runOptions struct {
	env   map[string]string
	stdin io.Reader
}

// RunOption configures runCommand execution.
type RunOption func(*runOptions)

// WithEnv applies the provided environment overrides when invoking a command.
func WithEnv(env map[string]string) RunOption {
	return func(o *runOptions) {
		if o.env == nil {
			o.env = make(map[string]string, len(env))
		}
		for k, v := range env {
			o.env[k] = v
		}
	}
}

// WithStdin configures an io.Reader to be used as stdin for the command.
func WithStdin(r io.Reader) RunOption {
	return func(o *runOptions) {
		o.stdin = r
	}
}

// runCommand executes the command and returns the combined stdout and stderr output.
func runCommand(t *testing.T, name string, args []string, opts ...RunOption) (string, error) {
	t.Helper()

	var cfg runOptions
	for _, opt := range opts {
		opt(&cfg)
	}

	cmd := exec.Command(name, args...)
	if cfg.stdin != nil {
		cmd.Stdin = cfg.stdin
	}

	if len(cfg.env) > 0 {
		env := os.Environ()
		for k, v := range cfg.env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	combined := combineOutput(stdout.Bytes(), stderr.Bytes())
	if err != nil {
		return combined, fmt.Errorf("%s %s: %w\n%s", name, strings.Join(args, " "), err, stderr.String())
	}

	return combined, nil
}

func combineOutput(stdout, stderr []byte) string {
	if len(stderr) == 0 {
		return string(stdout)
	}
	var buf bytes.Buffer
	buf.Grow(len(stdout) + len(stderr))
	buf.Write(stdout)
	buf.Write(stderr)
	return buf.String()
}

// requireTool skips the current test if the binary cannot be located on PATH.
func requireTool(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s not installed; skipping e2e", name)
	}
}

// ResultsRecorder captures a structured log of e2e steps for post-test analysis.
type ResultsRecorder struct {
	mu        sync.Mutex
	startedAt time.Time
	completed time.Time
	suite     string
	testName  string
	steps     []StepResult
	success   bool
	path      string
	once      sync.Once
	t         *testing.T
}

// StepResult describes the outcome of a single logical e2e step.
type StepResult struct {
	Name        string `json:"name"`
	StartedAt   string `json:"startedAt"`
	CompletedAt string `json:"completedAt"`
	DurationMS  int64  `json:"durationMs"`
	Success     bool   `json:"success"`
	Error       string `json:"error,omitempty"`
	Log         string `json:"log,omitempty"`
}

// SuiteResult represents the JSON structure persisted to disk for a single test.
type SuiteResult struct {
	Suite       string       `json:"suite"`
	Test        string       `json:"test"`
	StartedAt   string       `json:"startedAt"`
	CompletedAt string       `json:"completedAt"`
	DurationMS  int64        `json:"durationMs"`
	Success     bool         `json:"success"`
	Steps       []StepResult `json:"steps"`
}

// NewResultsRecorder initialises a recorder and registers a flush cleanup with the testing framework.
func NewResultsRecorder(t *testing.T, suite string) *ResultsRecorder {
	t.Helper()

	art := os.Getenv("ARTIFACTS_DIR")
	if art == "" {
		art = "artifacts"
	}
	base := filepath.Join(art, "e2e")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatalf("create artifacts dir: %v", err)
	}

	rec := &ResultsRecorder{
		suite:     suite,
		testName:  sanitizeName(t.Name()),
		startedAt: time.Now().UTC(),
		success:   true,
		path:      filepath.Join(base, fmt.Sprintf("%s-%s.json", suite, sanitizeName(t.Name()))),
		t:         t,
	}

	t.Cleanup(func() {
		rec.flush()
	})

	return rec
}

// Step records the outcome of fn without failing the test.
func (r *ResultsRecorder) Step(name string, fn func() (string, error)) error {
	started := time.Now().UTC()
	log, err := fn()
	finished := time.Now().UTC()

	r.mu.Lock()
	defer r.mu.Unlock()

	step := StepResult{
		Name:        name,
		StartedAt:   started.Format(time.RFC3339),
		CompletedAt: finished.Format(time.RFC3339),
		DurationMS:  finished.Sub(started).Milliseconds(),
		Success:     err == nil,
		Log:         truncateLog(log),
	}
	if err != nil {
		step.Error = err.Error()
		r.success = false
	}
	r.steps = append(r.steps, step)
	r.completed = finished
	return err
}

// MustStep records the outcome of fn and fails the test on error.
func (r *ResultsRecorder) MustStep(name string, fn func() (string, error)) {
	if err := r.Step(name, fn); err != nil {
		r.t.Fatalf("%s: %v", name, err)
	}
}

func (r *ResultsRecorder) flush() {
	r.once.Do(func() {
		r.mu.Lock()
		defer r.mu.Unlock()

		if r.completed.IsZero() {
			r.completed = time.Now().UTC()
		}

		payload := SuiteResult{
			Suite:       r.suite,
			Test:        r.testName,
			StartedAt:   r.startedAt.Format(time.RFC3339),
			CompletedAt: r.completed.Format(time.RFC3339),
			DurationMS:  r.completed.Sub(r.startedAt).Milliseconds(),
			Success:     r.success,
			Steps:       r.steps,
		}

		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			r.t.Logf("marshal results: %v", err)
			return
		}
		if err := os.WriteFile(r.path, data, 0o644); err != nil {
			r.t.Logf("write results: %v", err)
		}
	})
}

func sanitizeName(name string) string {
	lower := strings.ToLower(name)
	replacer := strings.NewReplacer("/", "_", " ", "_", "::", "_")
	return replacer.Replace(lower)
}

func truncateLog(in string) string {
	const limit = 4096
	if len(in) <= limit {
		return strings.TrimSpace(in)
	}
	return strings.TrimSpace(in[:limit]) + "…(truncated)"
}
