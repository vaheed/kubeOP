package testcase

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"kubeop/internal/config"
	"kubeop/internal/dns"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestCloudflareEnsureARecord_PropagatesErrorBody(t *testing.T) {
	original := http.DefaultTransport
	defer func() { http.DefaultTransport = original }()

	var call int
	http.DefaultTransport = roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		call++
		switch call {
		case 1:
			if r.Method != http.MethodGet {
				t.Fatalf("expected GET on first call, got %s", r.Method)
			}
			body := `{"result":[]}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		case 2:
			if r.Method != http.MethodPost {
				t.Fatalf("expected POST on second call, got %s", r.Method)
			}
			body := `{"success":false,"errors":[{"message":"invalid ip"}]}`
			return &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		default:
			t.Fatalf("unexpected request #%d", call)
			return nil, nil
		}
	})

	cfg := &config.Config{ExternalDNSProvider: "cloudflare", CFAPIToken: "token", CFZoneID: "zone"}
	prov := dns.NewProvider(cfg)
	cf, ok := prov.(*dns.Cloudflare)
	if !ok {
		t.Fatalf("expected Cloudflare provider, got %T", prov)
	}
	err := cf.EnsureARecord("app.example.com", "bad-ip", 120)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "invalid ip") {
		t.Fatalf("expected error message to include body snippet, got %q", msg)
	}
}
