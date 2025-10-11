package testcase

import (
    "strings"
    "testing"
    "kubeop/internal/util"
)

func TestSlugify_Basic(t *testing.T) {
    if got := util.Slugify("Hello, World!"); got != "hello-world" {
        t.Fatalf("want hello-world, got %q", got)
    }
}

func TestSlugify_EmptyFallback(t *testing.T) {
    if got := util.Slugify("   ###   "); got != "ns" {
        t.Fatalf("want ns, got %q", got)
    }
}

func TestSlugify_MaxLen(t *testing.T) {
    s := strings.Repeat("a", 100)
    got := util.Slugify(s)
    if len(got) != 63 {
        t.Fatalf("expected 63 chars, got %d", len(got))
    }
}

