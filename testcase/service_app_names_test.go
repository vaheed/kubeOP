package testcase

import (
	"strings"
	"testing"

	"kubeop/internal/service"
	"kubeop/internal/store"
)

func TestKubeNameForTest_AppendsStableSuffix(t *testing.T) {
	t.Parallel()

	got := service.KubeNameForTest("web-02", "12345678-1234-1234-1234-1234567890ab")
	want := "web-02-4b05db11c-mrncm"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
	if len(got) > 63 {
		t.Fatalf("expected kube name length <=63, got %d", len(got))
	}
}

func TestKubeNameForTest_TruncatesBaseToFit(t *testing.T) {
	t.Parallel()

	base := strings.Repeat("a", 70)
	got := service.KubeNameForTest(base, "12345678-1234-1234-1234-1234567890ab")
	if len(got) > 63 {
		t.Fatalf("expected kube name length <=63, got %d", len(got))
	}
	if !strings.HasSuffix(got, "-4b05db11c-mrncm") {
		t.Fatalf("expected suffix -4b05db11c-mrncm, got %q", got)
	}
}

func TestAppKubeNameForTest_PrefersStoredValue(t *testing.T) {
	t.Parallel()

	app := store.App{Name: "demo", Source: map[string]any{"kubeName": "demo-1234"}}
	if got := service.AppKubeNameForTest(app); got != "demo-1234" {
		t.Fatalf("expected stored kubeName demo-1234, got %q", got)
	}
}

func TestAppKubeNameForTest_FallbackSlug(t *testing.T) {
	t.Parallel()

	app := store.App{Name: "Hello World"}
	if got := service.AppKubeNameForTest(app); got != "hello-world" {
		t.Fatalf("expected fallback slug hello-world, got %q", got)
	}
}
