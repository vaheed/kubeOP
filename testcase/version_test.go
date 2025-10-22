package testcase

import (
	"strings"
	"testing"
	"time"

	"kubeop/internal/version"
)

func TestVersion_MetadataBumped(t *testing.T) {
	const expected = "0.9.2"
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

func TestVersion_SupportsClient(t *testing.T) {
	meta := version.Metadata()
	if !meta.SupportsClient("0.8.19") {
		t.Fatalf("expected client 0.8.19 to be supported")
	}
	if meta.SupportsClient("0.7.9") {
		t.Fatalf("expected client 0.7.9 to be rejected")
	}
	if !meta.SupportsClient("v0.8.18") {
		t.Fatalf("expected v-prefixed client to be normalised")
	}
}

func TestVersion_DeprecatedFalseWithoutDeadline(t *testing.T) {
	meta := version.Metadata()
	if meta.Deprecated(time.Now().UTC()) {
		t.Fatalf("expected metadata without deadline to be non-deprecated")
	}
}
