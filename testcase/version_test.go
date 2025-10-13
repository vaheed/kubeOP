package testcase

import (
	"strings"
	"testing"

	"kubeop/internal/version"
)

func TestVersion_Bumped(t *testing.T) {
	if version.Version != "0.4.0" {
		t.Fatalf("expected version 0.4.0, got %q", version.Version)
	}
}

func TestVersion_MetadataTrimmed(t *testing.T) {
	for name, value := range map[string]string{
		"Version": version.Version,
		"Commit":  version.Commit,
		"Date":    version.Date,
	} {
		if strings.TrimSpace(value) != value {
			t.Fatalf("%s has surrounding whitespace: %q", name, value)
		}
	}
}
