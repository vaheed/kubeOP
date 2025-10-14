package testcase

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kubeop/internal/config"
	kdns "kubeop/internal/dns"
)

func TestDNSProvider_None(t *testing.T) {
	cfg := &config.Config{}
	if p := kdns.NewProvider(cfg); p != nil {
		t.Fatalf("expected nil provider when EXTERNAL_DNS_PROVIDER unset")
	}
}

func TestDNSProvider_CloudflareSelection(t *testing.T) {
	cfg := &config.Config{ExternalDNSProvider: "cloudflare", CFAPIToken: "t", CFZoneID: "z"}
	if p := kdns.NewProvider(cfg); p == nil {
		t.Fatalf("expected cloudflare provider when token and zone id present")
	}
}

func TestDNSProvider_PowerDNSSelection(t *testing.T) {
	cfg := &config.Config{ExternalDNSProvider: "powerdns", PDNSAPIURL: "http://pdns:8081", PDNSAPIKey: "k", PDNSServerID: "localhost", PDNSZone: "example.com"}
	if p := kdns.NewProvider(cfg); p == nil {
		t.Fatalf("expected powerdns provider when api url and key present")
	}
}

func TestCloudflareEnsureARecordSurfacesAPIErrors(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			fmt.Fprint(w, `{"success":true,"result":[{"id":"rec-1","content":"1.1.1.1"}]}`)
		case http.MethodPut:
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"success":false,"errors":[{"code":1000,"message":"record invalid"}]}`)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cf := kdns.NewCloudflare("token", "zone")
	cf.SetAPIBaseURL(server.URL)
	cf.SetHTTPClient(server.Client())

	err := cf.EnsureARecord("app.example.com", "2.2.2.2", 300)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "code=1000") || !strings.Contains(err.Error(), "record invalid") {
		t.Fatalf("expected cloudflare error details, got %q", err.Error())
	}
}

func TestCloudflareDeleteARecordIncludesResponseBody(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			fmt.Fprint(w, `{"success":true,"result":[{"id":"rec-1"}]}`)
		case http.MethodDelete:
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"success":false,"errors":[{"message":"delete failed"}]}`)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cf := kdns.NewCloudflare("token", "zone")
	cf.SetAPIBaseURL(server.URL)
	cf.SetHTTPClient(server.Client())

	err := cf.DeleteARecord("app.example.com")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "delete failed") {
		t.Fatalf("expected response body snippet in error, got %q", err.Error())
	}
}
