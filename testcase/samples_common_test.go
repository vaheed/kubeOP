package testcase

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func sampleCommand(t *testing.T, script string) *exec.Cmd {
	t.Helper()
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("abs root: %v", err)
	}
	common := filepath.Join(root, "samples", "lib", "common.sh")
	cmd := exec.Command("bash", "-lc", fmt.Sprintf("source %q; %s", common, script))
	cmd.Dir = root
	cmd.Env = os.Environ()
	return cmd
}

func TestSamplesCommonLogging(t *testing.T) {
	cmd := sampleCommand(t, "log_step 'Bootstrap'; log_info 'Ready'")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("logging helpers failed: %v (output: %s)", err, string(out))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 log lines, got %d: %q", len(lines), lines)
	}
	linePattern := regexp.MustCompile(`^[0-9T:-]+Z \[(STEP|INFO)\] [A-Za-z0-9 ]+$`)
	for i, line := range lines {
		if !linePattern.MatchString(line) {
			t.Fatalf("line %d has unexpected format: %q", i, line)
		}
	}
}

func TestSamplesCommonRequireEnv(t *testing.T) {
	if err := sampleCommand(t, "require_env TEST_MISSING").Run(); err == nil {
		t.Fatalf("expected require_env to fail when variable is missing")
	}

	cmd := sampleCommand(t, "require_env TEST_PRESENT")
	cmd.Env = append(cmd.Env, "TEST_PRESENT=1")
	if err := cmd.Run(); err != nil {
		t.Fatalf("require_env failed for set variable: %v", err)
	}
}

func TestSamplesCommonRequireCommand(t *testing.T) {
	if err := sampleCommand(t, "require_command ls").Run(); err != nil {
		t.Fatalf("require_command should pass for ls: %v", err)
	}

	if err := sampleCommand(t, "require_command definitely-not-a-command").Run(); err == nil {
		t.Fatalf("expected require_command to fail for missing binary")
	}
}
