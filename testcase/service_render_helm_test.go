package testcase

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"kubeop/internal/service"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRenderHelmChartFromURLUsesSafeClient(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("NO_PROXY", "*")

	restoreHosts := service.SetHelmChartAllowedHosts([]string{"charts.example.com"})
	t.Cleanup(restoreHosts)

	chartBytes := buildTestHelmChartArchive(t)

	restoreResolver := service.SetHelmChartHostResolver(func(ctx context.Context, host string) ([]net.IP, error) {
		if host != "charts.example.com" {
			return nil, fmt.Errorf("unexpected host lookup: %s", host)
		}
		return []net.IP{net.ParseIP("198.51.100.10")}, nil
	})
	t.Cleanup(restoreResolver)

	var (
		requestedHost string
		requestedURL  string
	)
	fakeClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestedHost = req.URL.Hostname()
		if req.Host != "charts.example.com" {
			t.Fatalf("expected host header charts.example.com, got %s", req.Host)
		}
		requestedURL = req.URL.String()
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(chartBytes)),
			Header:     make(http.Header),
			Request:    req,
		}
		return resp, nil
	})}

	restoreClient := service.SetHelmChartHTTPClient(fakeClient)
	t.Cleanup(restoreClient)

	rendered, err := service.RenderHelmChartFromURLForTest(context.Background(), "https://charts.example.com/test chart-0.1.0.tgz?download=1#section", "release", "default", map[string]any{"replicaCount": 1})
	if err != nil {
		t.Fatalf("renderHelmChartFromURL returned error: %v", err)
	}
	if len(strings.TrimSpace(rendered)) == 0 {
		t.Fatalf("expected rendered manifests, got empty output")
	}
	if requestedHost != "charts.example.com" {
		t.Fatalf("expected request to charts.example.com, got %s", requestedHost)
	}
	if requestedURL != "https://charts.example.com/test%20chart-0.1.0.tgz?download=1" {
		t.Fatalf("expected sanitized request URL, got %s", requestedURL)
	}
}

func TestRenderHelmChartFromURLDialUsesValidatedAddress(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("NO_PROXY", "*")

	restoreHosts := service.SetHelmChartAllowedHosts([]string{"charts.example.com"})
	t.Cleanup(restoreHosts)

	restoreResolver := service.SetHelmChartHostResolver(func(ctx context.Context, host string) ([]net.IP, error) {
		if host != "charts.example.com" {
			return nil, fmt.Errorf("unexpected host lookup: %s", host)
		}
		return []net.IP{net.ParseIP("198.51.100.10")}, nil
	})
	t.Cleanup(restoreResolver)

	restoreClient := service.SetHelmChartHTTPClient(nil)
	t.Cleanup(restoreClient)

	var dialAddresses []string
	var unexpectedAddress string
	restoreDial := service.SetHelmChartDialFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
		dialAddresses = append(dialAddresses, address)
		if !strings.Contains(address, "198.51.100.10") && unexpectedAddress == "" {
			unexpectedAddress = address
		}
		return nil, errors.New("dial blocked for test")
	})
	t.Cleanup(restoreDial)

	_, err := service.RenderHelmChartFromURLForTest(context.Background(), "https://charts.example.com/testchart-0.1.0.tgz", "release", "default", nil)
	if err == nil || !strings.Contains(err.Error(), "dial blocked for test") {
		t.Fatalf("expected dial blocked error, got %v", err)
	}
	if unexpectedAddress != "" {
		t.Fatalf("dial address %q did not use validated ip", unexpectedAddress)
	}
	if len(dialAddresses) == 0 {
		t.Fatalf("expected at least one dial attempt")
	}
}

func TestRenderHelmChartFromURLRejectsDisallowedPort(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("NO_PROXY", "*")

	restoreHosts := service.SetHelmChartAllowedHosts([]string{"charts.example.com"})
	t.Cleanup(restoreHosts)

	restoreResolver := service.SetHelmChartHostResolver(func(ctx context.Context, host string) ([]net.IP, error) {
		switch host {
		case "charts.example.com":
			return []net.IP{net.ParseIP("198.51.100.10")}, nil
		default:
			return nil, fmt.Errorf("unexpected host lookup: %s", host)
		}
	})
	t.Cleanup(restoreResolver)

	cases := []struct {
		name string
		url  string
	}{
		{name: "https alt port", url: "https://charts.example.com:8443/testchart-0.1.0.tgz"},
		{name: "http alt port", url: "http://charts.example.com:8080/testchart-0.1.0.tgz"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := service.RenderHelmChartFromURLForTest(context.Background(), tc.url, "release", "default", nil)
			if err == nil {
				t.Fatalf("expected error for %s", tc.url)
			}
			if !strings.Contains(err.Error(), "port") {
				t.Fatalf("expected port error for %s, got %v", tc.url, err)
			}
		})
	}
}

func TestRenderHelmChartFromURLRejectsRelativePathSegments(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("NO_PROXY", "*")

	restoreHosts := service.SetHelmChartAllowedHosts([]string{"charts.example.com"})
	t.Cleanup(restoreHosts)

	restoreResolver := service.SetHelmChartHostResolver(func(ctx context.Context, host string) ([]net.IP, error) {
		if host != "charts.example.com" {
			return nil, fmt.Errorf("unexpected host lookup: %s", host)
		}
		return []net.IP{net.ParseIP("198.51.100.10")}, nil
	})
	t.Cleanup(restoreResolver)

	cases := []string{
		"https://charts.example.com/../testchart-0.1.0.tgz",
		"https://charts.example.com/%2e%2e/testchart-0.1.0.tgz",
		"https://charts.example.com/app/./testchart-0.1.0.tgz",
	}

	for _, raw := range cases {
		raw := raw
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			_, err := service.RenderHelmChartFromURLForTest(context.Background(), raw, "release", "default", nil)
			if err == nil {
				t.Fatalf("expected error for %s", raw)
			}
			if !strings.Contains(err.Error(), "path") {
				t.Fatalf("expected path validation error for %s, got %v", raw, err)
			}
		})
	}
}

func TestRenderHelmChartFromURLRejectsDisallowedHost(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("NO_PROXY", "*")

	restoreHosts := service.SetHelmChartAllowedHosts([]string{"charts.example.com"})
	t.Cleanup(restoreHosts)

	_, err := service.RenderHelmChartFromURLForTest(context.Background(), "https://other.example.net/testchart-0.1.0.tgz", "release", "default", nil)
	if err == nil {
		t.Fatalf("expected host allow-list error")
	}
	if !strings.Contains(err.Error(), "not permitted") {
		t.Fatalf("expected not permitted error, got %v", err)
	}
}

func TestRenderHelmChartFromURLRejectsWhenAllowListEmpty(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("NO_PROXY", "*")

	restoreHosts := service.SetHelmChartAllowedHosts(nil)
	t.Cleanup(restoreHosts)

	_, err := service.RenderHelmChartFromURLForTest(context.Background(), "https://charts.example.com/testchart-0.1.0.tgz", "release", "default", nil)
	if err == nil {
		t.Fatalf("expected allow-list empty error")
	}
	if !strings.Contains(err.Error(), "allow-list is empty") {
		t.Fatalf("expected allow-list empty error, got %v", err)
	}
}

func buildTestHelmChartArchive(t *testing.T) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)

	writeFile := func(name, content string) {
		hdr := &tar.Header{
			Name:    name,
			Mode:    0o644,
			Size:    int64(len(content)),
			ModTime: time.Unix(0, 0),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header %s: %v", name, err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("write content %s: %v", name, err)
		}
	}

	chartDir := "testchart"
	writeFile(chartDir+"/Chart.yaml", "apiVersion: v2\nname: testchart\nversion: 0.1.0\n")
	writeFile(chartDir+"/values.yaml", "replicaCount: 1\n")
	writeFile(chartDir+"/templates/deployment.yaml", "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test-config\ndata:\n  value: {{ .Values.replicaCount | quote }}\n")

	if err := tw.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}

	return buf.Bytes()
}
