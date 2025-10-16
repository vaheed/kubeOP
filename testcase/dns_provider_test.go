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
	cfg := &config.Config{DNSAPIURL: "https://dns.example.com", DNSAPIKey: "key", DNSProvider: "http"}
	if p := kdns.NewProvider(cfg); p == nil {
		t.Fatalf("expected http provider when DNS_API_URL and DNS_API_KEY present")
	}
}

func TestDNSProvider_CloudflareSelection(t *testing.T) {
	cfg := &config.Config{DNSProvider: "cloudflare", CloudflareAPIBase: "https://api.cloudflare.com", CloudflareAPIToken: "token", CloudflareZoneID: "zone"}
	if p := kdns.NewProvider(cfg); p == nil {
		t.Fatalf("expected cloudflare provider when token and zone configured")
	}
}

func TestDNSProvider_PowerDNSSelection(t *testing.T) {
	cfg := &config.Config{DNSProvider: "powerdns", PowerDNSAPIURL: "https://dns.internal", PowerDNSAPIKey: "key", PowerDNSServerID: "localhost", PowerDNSZone: "example.com."}
	if p := kdns.NewProvider(cfg); p == nil {
		t.Fatalf("expected powerdns provider when api url/key/zone configured")
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

	cfg := &config.Config{DNSProvider: "http", DNSAPIURL: server.URL, DNSAPIKey: "secret", DNSRecordTTL: 120}
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

	cfg := &config.Config{DNSProvider: "http", DNSAPIURL: server.URL, DNSAPIKey: "secret"}
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

	cfg := &config.Config{DNSProvider: "http", DNSAPIURL: server.URL, DNSAPIKey: "secret"}
	provider := kdns.NewProvider(cfg).(*kdns.HTTPProvider)
	provider.SetHTTPClient(server.Client())

	err := provider.DeleteRecords("app.example.com")
	if err == nil || err.Error() != "dns api delete failed: 500 Internal Server Error (delete failed)" {
		t.Fatalf("expected detailed error, got %v", err)
	}
}

func TestCloudflareEnsureRecordsCreatesMissingEntries(t *testing.T) {
	t.Parallel()

	type call struct {
		Method string
		Path   string
		Body   map[string]any
	}
	calls := make([]call, 0, 3)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		switch r.Method {
		case http.MethodGet:
			calls = append(calls, call{Method: r.Method, Path: r.URL.Path})
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result":  []map[string]any{},
			})
		case http.MethodPost:
			body := map[string]any{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			calls = append(calls, call{Method: r.Method, Path: r.URL.Path, Body: body})
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "result": map[string]any{"id": "123"}})
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	t.Cleanup(server.Close)

	cfg := &config.Config{
		DNSProvider:        "cloudflare",
		CloudflareAPIBase:  server.URL,
		CloudflareAPIToken: "token",
		CloudflareZoneID:   "zone",
		DNSRecordTTL:       180,
	}

	provider := kdns.NewProvider(cfg)
	cloudflare, ok := provider.(*kdns.CloudflareProvider)
	if !ok {
		t.Fatalf("expected CloudflareProvider, got %T", provider)
	}
	cloudflare.SetHTTPClient(server.Client())

	addrs := []netip.Addr{netip.MustParseAddr("10.0.0.10"), netip.MustParseAddr("2001:db8::1")}
	if err := cloudflare.EnsureRecords("app.example.com", addrs, cfg.DNSRecordTTL); err != nil {
		t.Fatalf("ensure records: %v", err)
	}

	if len(calls) != 3 {
		t.Fatalf("expected three calls (GET + two POST), got %d", len(calls))
	}
	if calls[0].Method != http.MethodGet {
		t.Fatalf("expected first call GET, got %s", calls[0].Method)
	}
	for i := 1; i < len(calls); i++ {
		if calls[i].Method != http.MethodPost {
			t.Fatalf("expected POST call, got %s", calls[i].Method)
		}
		if calls[i].Body["name"].(string) != "app.example.com" {
			t.Fatalf("expected host name in payload, got %v", calls[i].Body["name"])
		}
		if int(calls[i].Body["ttl"].(float64)) != 180 {
			t.Fatalf("expected TTL 180, got %v", calls[i].Body["ttl"])
		}
	}
}

func TestCloudflareDeleteRecordsRemovesExisting(t *testing.T) {
	t.Parallel()

	deletes := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result": []map[string]any{
					{"id": "a", "type": "A", "name": "app.example.com", "content": "10.0.0.1"},
					{"id": "b", "type": "AAAA", "name": "app.example.com", "content": "2001:db8::1"},
				},
			})
		case http.MethodDelete:
			deletes++
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true})
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	}))
	t.Cleanup(server.Close)

	cfg := &config.Config{
		DNSProvider:        "cloudflare",
		CloudflareAPIBase:  server.URL,
		CloudflareAPIToken: "token",
		CloudflareZoneID:   "zone",
	}

	provider := kdns.NewProvider(cfg)
	cloudflare := provider.(*kdns.CloudflareProvider)
	cloudflare.SetHTTPClient(server.Client())

	if err := cloudflare.DeleteRecords("app.example.com"); err != nil {
		t.Fatalf("delete records: %v", err)
	}
	if deletes != 2 {
		t.Fatalf("expected two delete calls, got %d", deletes)
	}
}

func TestPowerDNSEnsureRecordsBuildsRRSet(t *testing.T) {
	t.Parallel()

	var payload powerdnsRequestCapture
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		defer r.Body.Close()
		_ = json.NewDecoder(r.Body).Decode(&payload)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	cfg := &config.Config{
		DNSProvider:      "powerdns",
		PowerDNSAPIURL:   server.URL,
		PowerDNSAPIKey:   "secret",
		PowerDNSServerID: "localhost",
		PowerDNSZone:     "example.com.",
		DNSRecordTTL:     200,
	}

	provider := kdns.NewProvider(cfg)
	powerdns := provider.(*kdns.PowerDNSProvider)
	powerdns.SetHTTPClient(server.Client())

	addrs := []netip.Addr{netip.MustParseAddr("10.0.0.2"), netip.MustParseAddr("2001:db8::2")}
	if err := powerdns.EnsureRecords("app.example.com", addrs, cfg.DNSRecordTTL); err != nil {
		t.Fatalf("ensure records: %v", err)
	}

	if len(payload.RRSets) != 2 {
		t.Fatalf("expected two rrsets, got %d", len(payload.RRSets))
	}
	if payload.RRSets[0].Name != "app.example.com." {
		t.Fatalf("expected fqdn with trailing dot, got %s", payload.RRSets[0].Name)
	}
}

func TestPowerDNSDeleteRecordsSendsDeleteRRSets(t *testing.T) {
	t.Parallel()

	var payload powerdnsRequestCapture
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		defer r.Body.Close()
		_ = json.NewDecoder(r.Body).Decode(&payload)
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	cfg := &config.Config{
		DNSProvider:      "powerdns",
		PowerDNSAPIURL:   server.URL,
		PowerDNSAPIKey:   "secret",
		PowerDNSServerID: "localhost",
		PowerDNSZone:     "example.com.",
	}

	provider := kdns.NewProvider(cfg)
	powerdns := provider.(*kdns.PowerDNSProvider)
	powerdns.SetHTTPClient(server.Client())

	if err := powerdns.DeleteRecords("app.example.com"); err != nil {
		t.Fatalf("delete records: %v", err)
	}
	if len(payload.RRSets) != 2 {
		t.Fatalf("expected two delete rrsets, got %d", len(payload.RRSets))
	}
	if payload.RRSets[0].ChangeType != "DELETE" {
		t.Fatalf("expected DELETE change type, got %s", payload.RRSets[0].ChangeType)
	}
}

type powerdnsRequestCapture struct {
	RRSets []struct {
		Name       string `json:"name"`
		Type       string `json:"type"`
		TTL        int    `json:"ttl"`
		ChangeType string `json:"changetype"`
		Records    []struct {
			Content  string `json:"content"`
			Disabled bool   `json:"disabled"`
		} `json:"records"`
	} `json:"rrsets"`
}
