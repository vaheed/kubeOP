package testcase

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"testing"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/registry"

	"kubeop/internal/service"
)

type stubRegistryClient struct {
	expectedRef string
	chartBytes  []byte
	pullErr     error
	loginErr    error

	pullCalls  int
	loginCalls int
	loginHost  string
}

func (s *stubRegistryClient) Pull(ref string, _ ...registry.PullOption) (*registry.PullResult, error) {
	s.pullCalls++
	if s.pullErr != nil {
		return nil, s.pullErr
	}
	if s.expectedRef != "" && s.expectedRef != ref {
		return nil, fmt.Errorf("unexpected ref %s", ref)
	}
	if len(s.chartBytes) == 0 {
		return nil, fmt.Errorf("chart data not set")
	}
	return &registry.PullResult{
		Chart: &registry.DescriptorPullSummaryWithMeta{
			DescriptorPullSummary: registry.DescriptorPullSummary{
				Digest: "sha256:test",
				Size:   int64(len(s.chartBytes)),
				Data:   s.chartBytes,
			},
			Meta: &chart.Metadata{Name: "testchart", Version: "0.1.0"},
		},
	}, nil
}

func (s *stubRegistryClient) Login(host string, _ ...registry.LoginOption) error {
	s.loginCalls++
	s.loginHost = host
	return s.loginErr
}

func TestRenderHelmChartFromOCI(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("NO_PROXY", "*")

	chartBytes := buildTestHelmChartArchive(t)
	restoreResolver := service.SetHelmChartHostResolver(func(ctx context.Context, host string) ([]net.IP, error) {
		if host != "registry.example.com" {
			return nil, fmt.Errorf("unexpected host lookup: %s", host)
		}
		return []net.IP{net.ParseIP("198.51.100.10")}, nil
	})
	t.Cleanup(restoreResolver)

	stub := &stubRegistryClient{expectedRef: "oci://registry.example.com/library/chart:1.0.0", chartBytes: chartBytes}
	var capturedHost string
	var capturedAddrs []netip.Addr
	var capturedInsecure bool
	restoreFactory := service.SetHelmRegistryClientFactory(func(host string, addrs []netip.Addr, insecure bool) (service.HelmRegistryClient, error) {
		capturedHost = host
		capturedAddrs = append([]netip.Addr(nil), addrs...)
		capturedInsecure = insecure
		return stub, nil
	})
	t.Cleanup(restoreFactory)

	rendered, err := service.RenderHelmChartFromOCIForTest(context.Background(), "oci://registry.example.com/library/chart:1.0.0", "release", "default", map[string]any{"replicaCount": 1}, "", "", false)
	if err != nil {
		t.Fatalf("RenderHelmChartFromOCIForTest returned error: %v", err)
	}
	if len(rendered) == 0 {
		t.Fatalf("expected rendered manifests")
	}
	if stub.pullCalls != 1 {
		t.Fatalf("expected pull to be called once, got %d", stub.pullCalls)
	}
	if stub.loginCalls != 0 {
		t.Fatalf("expected login to be skipped, got %d", stub.loginCalls)
	}
	if capturedHost != "registry.example.com" {
		t.Fatalf("expected factory host registry.example.com, got %s", capturedHost)
	}
	if capturedInsecure {
		t.Fatalf("expected insecure to be false")
	}
	expectedAddr := netip.MustParseAddr("198.51.100.10")
	if len(capturedAddrs) != 1 || capturedAddrs[0] != expectedAddr {
		t.Fatalf("unexpected allowed addresses: %#v", capturedAddrs)
	}
}

func TestRenderHelmChartFromOCIWithAuth(t *testing.T) {
	chartBytes := buildTestHelmChartArchive(t)
	restoreResolver := service.SetHelmChartHostResolver(func(ctx context.Context, host string) ([]net.IP, error) {
		return []net.IP{net.ParseIP("198.51.100.20")}, nil
	})
	t.Cleanup(restoreResolver)

	stub := &stubRegistryClient{expectedRef: "oci://registry.example.com/app/chart:2.0.0", chartBytes: chartBytes}
	var capturedInsecure bool
	restoreFactory := service.SetHelmRegistryClientFactory(func(host string, addrs []netip.Addr, insecure bool) (service.HelmRegistryClient, error) {
		capturedInsecure = insecure
		return stub, nil
	})
	t.Cleanup(restoreFactory)

	if _, err := service.RenderHelmChartFromOCIForTest(context.Background(), "oci://registry.example.com/app/chart:2.0.0", "rel", "ns", nil, "user", "pass", true); err != nil {
		t.Fatalf("RenderHelmChartFromOCIForTest returned error: %v", err)
	}
	if stub.loginCalls != 1 {
		t.Fatalf("expected login to run once, got %d", stub.loginCalls)
	}
	if stub.loginHost != "registry.example.com" {
		t.Fatalf("expected login host registry.example.com, got %s", stub.loginHost)
	}
	if !capturedInsecure {
		t.Fatalf("expected insecure factory flag true")
	}
}

func TestRenderHelmChartFromOCIInvalidRef(t *testing.T) {
	if _, err := service.RenderHelmChartFromOCIForTest(context.Background(), "oci://registry.example.com/chart", "rel", "ns", nil, "", "", false); err == nil {
		t.Fatalf("expected error for ref without tag")
	}
}
