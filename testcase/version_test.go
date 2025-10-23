package testcase

import (
	"strings"
	"testing"

	"kubeop/internal/version"
)

func TestVersion_MetadataBumped(t *testing.T) {
	const expected = "0.15.2"
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

func TestVersion_FromStringsFallsBackForInvalidSemver(t *testing.T) {
	info, err := version.FromStrings("dev", "", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got, want := info.Build.Version, "0.0.0-dev"; got != want {
		t.Fatalf("expected fallback version %q, got %q", want, got)
	}
}
