package testcase

import (
	"strings"
	"testing"

	"kubeop/internal/version"
)

func TestVersion_Bumped(t *testing.T) {
	const expected = "0.12.0"
	if version.Version != expected {
		t.Fatalf("expected version %s, got %q", expected, version.Version)
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
