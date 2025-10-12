package testcase

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
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
	t.Parallel()

	chartBytes := buildTestHelmChartArchive(t)

	restoreResolver := service.SetHelmChartHostResolver(func(ctx context.Context, host string) ([]net.IP, error) {
		if host != "charts.example.com" {
			return nil, fmt.Errorf("unexpected host lookup: %s", host)
		}
		return []net.IP{net.ParseIP("198.51.100.10")}, nil
	})
	t.Cleanup(restoreResolver)

	var requestedHost string
	fakeClient := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		requestedHost = req.URL.Hostname()
		if req.Host != "charts.example.com" {
			t.Fatalf("expected host header charts.example.com, got %s", req.Host)
		}
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

	rendered, err := service.RenderHelmChartFromURLForTest(context.Background(), "https://charts.example.com/testchart-0.1.0.tgz", "release", "default", map[string]any{"replicaCount": 1})
	if err != nil {
		t.Fatalf("renderHelmChartFromURL returned error: %v", err)
	}
	if len(strings.TrimSpace(rendered)) == 0 {
		t.Fatalf("expected rendered manifests, got empty output")
	}
	if requestedHost != "charts.example.com" {
		t.Fatalf("expected request to charts.example.com, got %s", requestedHost)
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
