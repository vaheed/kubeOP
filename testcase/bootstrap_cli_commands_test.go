package testcase

import (
	"bytes"
	"io"
	"testing"

	appv1alpha1 "github.com/vaheed/kubeOP/kubeop-operator/apis/paas/v1alpha1"
	bootstrapcli "github.com/vaheed/kubeOP/kubeop-operator/pkg/bootstrapcli"
)

func TestBootstrapProjectCommandDefaultEnvironment(t *testing.T) {
	root := bootstrapcli.NewCommand(bootstrapcli.IOStreams{In: bytes.NewReader(nil), Out: io.Discard, ErrOut: io.Discard})
	projectCmd, _, err := root.Find([]string{"project", "create"})
	if err != nil {
		t.Fatalf("expected project create command: %v", err)
	}
	flag := projectCmd.Flag("environment")
	if flag == nil {
		t.Fatalf("environment flag not found")
	}
	if flag.DefValue != string(appv1alpha1.ProjectEnvironmentDev) {
		t.Fatalf("expected default environment %q, got %q", appv1alpha1.ProjectEnvironmentDev, flag.DefValue)
	}
}

func TestBootstrapIncludesInitCommand(t *testing.T) {
	root := bootstrapcli.NewCommand(bootstrapcli.IOStreams{In: bytes.NewReader(nil), Out: io.Discard, ErrOut: io.Discard})
	cmd, _, err := root.Find([]string{"init"})
	if err != nil {
		t.Fatalf("init command missing: %v", err)
	}
	if cmd.Short == "" {
		t.Fatalf("init command should have summary")
	}
}
