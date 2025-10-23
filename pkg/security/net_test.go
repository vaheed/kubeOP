package security_test

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"testing"

	"kubeop/pkg/security"
)

func TestAllowedHost(t *testing.T) {
	t.Parallel()
	allowed := []string{"charts.example.com", "*.trusted.local"}
	if !security.AllowedHost("charts.example.com", allowed) {
		t.Fatalf("expected charts.example.com to be allowed")
	}
	if !security.AllowedHost("apps.trusted.local", allowed) {
		t.Fatalf("expected wildcard host to be allowed")
	}
	if security.AllowedHost("evil.example.com", allowed) {
		t.Fatalf("unexpected host permitted")
	}
}

func TestHasDotDot(t *testing.T) {
	t.Parallel()
	if !security.HasDotDot("../foo") {
		t.Fatalf("expected HasDotDot to detect traversal")
	}
	if security.HasDotDot("/charts/app") {
		t.Fatalf("did not expect traversal for clean path")
	}
}

func TestParseAndValidateHTTPSURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		raw     string
		wantErr bool
	}{
		{name: "valid", raw: "https://charts.example.com/releases/app.tgz"},
		{name: "reject http", raw: "http://charts.example.com/app.tgz", wantErr: true},
		{name: "reject credentials", raw: "https://user@charts.example.com/app.tgz", wantErr: true},
		{name: "reject ip literal", raw: "https://192.0.2.10/app.tgz", wantErr: true},
		{name: "reject raw dotdot", raw: "https://charts.example.com/../../app.tgz", wantErr: true},
		{name: "reject encoded dotdot", raw: "https://charts.example.com/%2e%2e/app.tgz", wantErr: true},
		{name: "reject fragment", raw: "https://charts.example.com/app.tgz#section", wantErr: false},
		{name: "reject port", raw: "https://charts.example.com:8443/app.tgz", wantErr: true},
	}
	allow := func(host string) bool { return host == "charts.example.com" }
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			u, err := security.ParseAndValidateHTTPSURL(tc.raw, allow)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tc.raw)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.raw, err)
			}
			if u.Scheme != "https" {
				t.Fatalf("expected https scheme, got %s", u.Scheme)
			}
		})
	}
}

func TestValidateRedirect(t *testing.T) {
	t.Parallel()
	req := &http.Request{URL: mustParse(t, "https://charts.example.com/app.tgz")}
	via := []*http.Request{{URL: mustParse(t, "https://charts.example.com/app.tgz")}}
	if err := security.ValidateRedirect(req, via, func(host string) bool { return host == "charts.example.com" }); err != nil {
		t.Fatalf("unexpected error validating redirect: %v", err)
	}

	badReq := &http.Request{URL: mustParse(t, "https://evil.example.com/app.tgz")}
	if err := security.ValidateRedirect(badReq, via, func(host string) bool { return host == "charts.example.com" }); err == nil {
		t.Fatalf("expected redirect validation to fail for disallowed host")
	}
}

func TestDenyPrivateNetworks(t *testing.T) {
	t.Parallel()
	dialed := false
	base := func(ctx context.Context, network, address string) (net.Conn, error) {
		dialed = true
		return nil, context.Canceled
	}
	deny := security.DenyPrivateNetworks(base)
	if _, err := deny(context.Background(), "tcp", "10.0.0.1:443"); err == nil {
		t.Fatalf("expected private network dial to fail")
	}
	if dialed {
		t.Fatalf("base dialer should not be invoked for private ip")
	}
	_, err := deny(context.Background(), "tcp", "198.51.100.10:443")
	if err == nil {
		t.Fatalf("expected base dialer error to propagate")
	}
}

func TestIsPublicAddr(t *testing.T) {
	t.Parallel()
	public := netip.MustParseAddr("198.51.100.10")
	private := netip.MustParseAddr("10.0.0.1")
	if !security.IsPublicAddr(public) {
		t.Fatalf("expected %s to be public", public)
	}
	if security.IsPublicAddr(private) {
		t.Fatalf("expected %s to be private", private)
	}
}

func mustParse(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	return u
}

func FuzzParseAndValidateHTTPSURL(f *testing.F) {
	f.Add("https://charts.example.com/app.tgz")
	f.Add("http://invalid")
	allow := func(host string) bool { return host == "charts.example.com" }
	f.Fuzz(func(t *testing.T, raw string) {
		_, _ = security.ParseAndValidateHTTPSURL(raw, allow)
	})
}
