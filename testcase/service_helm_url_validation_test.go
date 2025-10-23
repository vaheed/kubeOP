package testcase

import (
	"context"
	"errors"
	"net"
	"testing"

	"kubeop/internal/service"
)

func TestValidateHelmChartURL(t *testing.T) {
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("NO_PROXY", "*")

	restoreHosts := service.SetHelmChartAllowedHosts([]string{"charts.example.com"})
	t.Cleanup(restoreHosts)

	restore := service.SetHelmChartHostResolver(func(ctx context.Context, host string) ([]net.IP, error) {
		switch host {
		case "charts.example.com":
			return []net.IP{net.ParseIP("198.51.100.10")}, nil
		case "private.example.com":
			return []net.IP{net.ParseIP("10.0.0.5")}, nil
		case "localhost":
			return []net.IP{net.ParseIP("127.0.0.1")}, nil
		default:
			return nil, errors.New("host not stubbed")
		}
	})
	t.Cleanup(restore)

	cases := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "https ok", input: "https://charts.example.com/grafana-1.2.3.tgz"},
		{name: "http ok", input: "http://charts.example.com/app.tgz"},
		{name: "https explicit port ok", input: "https://charts.example.com:443/grafana-1.2.3.tgz"},
		{name: "http explicit port ok", input: "http://charts.example.com:80/app.tgz"},
		{name: "reject file scheme", input: "file:///etc/passwd", wantErr: true},
		{name: "reject localhost", input: "https://localhost/chart.tgz", wantErr: true},
		{name: "reject loopback ip", input: "https://127.0.0.1/chart.tgz", wantErr: true},
		{name: "reject private ip", input: "https://10.0.0.5/chart.tgz", wantErr: true},
		{name: "reject credentials", input: "https://user@example.com/chart.tgz", wantErr: true},
		{name: "reject hostname resolving private", input: "https://private.example.com/chart.tgz", wantErr: true},
		{name: "reject https alt port", input: "https://charts.example.com:8443/chart.tgz", wantErr: true},
		{name: "reject http alt port", input: "http://charts.example.com:8080/chart.tgz", wantErr: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := service.ValidateHelmChartURL(context.Background(), tc.input)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tc.input)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.input, err)
			}
		})
	}
}
