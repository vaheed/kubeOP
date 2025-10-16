package testcase

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"kubeop/internal/config"
	kdns "kubeop/internal/dns"
)

func TestDNSProvider_None(t *testing.T) {
	cfg := &config.Config{}
	if p := kdns.NewProvider(cfg); p != nil {
		t.Fatalf("expected nil provider when DNS_API_URL unset")
	}
}

func TestDNSProvider_HTTPSelection(t *testing.T) {
	cfg := &config.Config{DNSAPIURL: "https://dns.example.com", DNSAPIKey: "key"}
	if p := kdns.NewProvider(cfg); p == nil {
		t.Fatalf("expected http provider when DNS_API_URL and DNS_API_KEY present")
	}
}

func TestHTTPProviderEnsureRecordsSendsPayload(t *testing.T) {
	t.Parallel()

	var received struct {
		Method string
		Path   string
		Auth   string
		Body   map[string]any
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Method = r.Method
		received.Path = r.URL.Path
		received.Auth = r.Header.Get("Authorization")
		defer r.Body.Close()
		_ = json.NewDecoder(r.Body).Decode(&received.Body)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	cfg := &config.Config{DNSAPIURL: server.URL, DNSAPIKey: "secret", DNSRecordTTL: 120}
	provider := kdns.NewProvider(cfg)
	httpProvider, ok := provider.(*kdns.HTTPProvider)
	if !ok {
		t.Fatalf("expected HTTPProvider, got %T", provider)
	}
	httpProvider.SetHTTPClient(server.Client())

	addrs := []netip.Addr{netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("2001:db8::1")}
	if err := httpProvider.EnsureRecords("app.example.com", addrs, cfg.DNSRecordTTL); err != nil {
		t.Fatalf("ensure records: %v", err)
	}

	if received.Method != http.MethodPut {
		t.Fatalf("expected PUT request, got %s", received.Method)
	}
	if received.Path != "/records" {
		t.Fatalf("expected /records path, got %s", received.Path)
	}
	if received.Auth != "Bearer secret" {
		t.Fatalf("expected Authorization header, got %s", received.Auth)
	}
	if received.Body["fqdn"].(string) != "app.example.com" {
		t.Fatalf("expected fqdn app.example.com, got %v", received.Body["fqdn"])
	}
	if int(received.Body["ttl"].(float64)) != 120 {
		t.Fatalf("expected ttl 120, got %v", received.Body["ttl"])
	}
	records, ok := received.Body["records"].([]any)
	if !ok || len(records) != 2 {
		t.Fatalf("expected two records, got %#v", received.Body["records"])
	}
}

func TestHTTPProviderEnsureRecordsSurfacesErrors(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "upstream failed"})
	}))
	t.Cleanup(server.Close)

	cfg := &config.Config{DNSAPIURL: server.URL, DNSAPIKey: "secret"}
	provider := kdns.NewProvider(cfg).(*kdns.HTTPProvider)
	provider.SetHTTPClient(server.Client())

	err := provider.EnsureRecords("app.example.com", []netip.Addr{netip.MustParseAddr("10.0.0.1")}, 60)
	if err == nil || err.Error() != "dns api ensure failed: 502 Bad Gateway (upstream failed)" {
		t.Fatalf("expected detailed error, got %v", err)
	}
}

func TestHTTPProviderDeleteRecordsSurfacesErrors(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "delete failed"})
	}))
	t.Cleanup(server.Close)

	cfg := &config.Config{DNSAPIURL: server.URL, DNSAPIKey: "secret"}
	provider := kdns.NewProvider(cfg).(*kdns.HTTPProvider)
	provider.SetHTTPClient(server.Client())

	err := provider.DeleteRecords("app.example.com")
	if err == nil || err.Error() != "dns api delete failed: 500 Internal Server Error (delete failed)" {
		t.Fatalf("expected detailed error, got %v", err)
	}
}
