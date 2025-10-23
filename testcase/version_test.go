package testcase

import (
	"strings"
	"testing"

	"kubeop/internal/version"
)

func TestVersion_MetadataBumped(t *testing.T) {
	const expected = "0.15.0"
	meta := version.Metadata()
	if meta.Build.Version != expected {
		t.Fatalf("expected build version %s, got %q", expected, meta.Build.Version)
	}
	for name, value := range map[string]string{
		"Version": meta.Build.Version,
		"Commit":  meta.Build.Commit,
		"Date":    meta.Build.Date,
	} {
		if strings.TrimSpace(value) != value {
			t.Fatalf("%s has surrounding whitespace: %q", name, value)
		}
	}
}
