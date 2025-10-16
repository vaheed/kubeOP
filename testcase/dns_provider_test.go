package testcase

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"sync"
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

func TestDNSProvider_CloudflareSelection(t *testing.T) {
	cfg := &config.Config{ExternalDNSProvider: "cloudflare", CFAPIToken: "token", CFZoneID: "zone"}
	if p := kdns.NewProvider(cfg); p == nil {
		t.Fatalf("expected http provider when DNS_API_URL and DNS_API_KEY present")
	}
}

func TestDNSProvider_PowerDNSSelection(t *testing.T) {
	cfg := &config.Config{ExternalDNSProvider: "powerdns", PDNSAPIURL: "http://pdns", PDNSAPIKey: "key", PDNSServerID: "localhost", PDNSZone: "example.com"}
	if p := kdns.NewProvider(cfg); p == nil {
		t.Fatalf("expected powerdns provider when api url and key present")
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

func TestCloudflareEnsureRecordsCreatesIPv4AndIPv6(t *testing.T) {
	t.Parallel()

	var mu sync.Mutex
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.RawQuery, "type=A"):
			fmt.Fprint(w, `{"success":true,"result":[]}`)
		case r.Method == http.MethodGet && strings.Contains(r.URL.RawQuery, "type=AAAA"):
			fmt.Fprint(w, `{"success":true,"result":[]}`)
		case r.Method == http.MethodPost && strings.Contains(r.URL.Path, "/dns_records"):
			body, _ := io.ReadAll(r.Body)
			defer r.Body.Close()
			var payload map[string]any
			_ = json.Unmarshal(body, &payload)
			requests = append(requests, payload)
			fmt.Fprint(w, `{"success":true,"result":{}}`)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	}))
	t.Cleanup(server.Close)

	cf := kdns.NewCloudflare("token", "zone")
	cf.SetAPIBaseURL(server.URL)
	cf.SetHTTPClient(server.Client())

	addrs := []netip.Addr{netip.MustParseAddr("192.0.2.10"), netip.MustParseAddr("2001:db8::1")}
	if err := cf.EnsureRecords("app.example.com", addrs, 120); err != nil {
		t.Fatalf("ensure records: %v", err)
	}

	if len(requests) != 2 {
		t.Fatalf("expected two create requests, got %d", len(requests))
	}
	if requests[0]["type"].(string) != "A" || requests[0]["content"].(string) != "192.0.2.10" {
		t.Fatalf("unexpected ipv4 payload: %#v", requests[0])
	}
	if requests[1]["type"].(string) != "AAAA" || requests[1]["content"].(string) != "2001:db8::1" {
		t.Fatalf("unexpected ipv6 payload: %#v", requests[1])
	}
	if int(requests[0]["ttl"].(float64)) != 120 || int(requests[1]["ttl"].(float64)) != 120 {
		t.Fatalf("expected ttl 120 in payloads: %#v", requests)
	}
}

func TestCloudflareEnsureRecordsSurfacesAPIErrors(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			fmt.Fprint(w, `{"success":true,"result":[{"id":"rec-1","content":"1.1.1.1"}]}`)
		case http.MethodPut:
			fmt.Fprint(w, `{"success":false,"errors":[{"code":1000,"message":"record invalid"}]}`)
		default:
			t.Fatalf("unexpected method %s", r.Method)
		}
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := &config.Config{DNSAPIURL: server.URL, DNSAPIKey: "secret"}
	provider := kdns.NewProvider(cfg).(*kdns.HTTPProvider)
	provider.SetHTTPClient(server.Client())

	err := cf.EnsureRecords("app.example.com", []netip.Addr{netip.MustParseAddr("2.2.2.2")}, 300)
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "code=1000") || !strings.Contains(err.Error(), "record invalid") {
		t.Fatalf("expected cloudflare error details, got %q", err.Error())
	}
}

func TestCloudflareDeleteRecordsIncludesResponseBody(t *testing.T) {
	t.Parallel()

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.Contains(r.URL.RawQuery, "type=A"):
			fmt.Fprint(w, `{"success":true,"result":[{"id":"rec-a"}]}`)
		case r.Method == http.MethodGet && strings.Contains(r.URL.RawQuery, "type=AAAA"):
			fmt.Fprint(w, `{"success":true,"result":[{"id":"rec-aaaa"}]}`)
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "rec-a"):
			w.WriteHeader(http.StatusInternalServerError)
			fmt.Fprint(w, `{"success":false,"errors":[{"message":"delete failed"}]}`)
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "rec-aaaa"):
			fmt.Fprint(w, `{"success":true,"result":{}}`)
		default:
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.String())
		}
	})
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	cfg := &config.Config{DNSAPIURL: server.URL, DNSAPIKey: "secret"}
	provider := kdns.NewProvider(cfg).(*kdns.HTTPProvider)
	provider.SetHTTPClient(server.Client())

	err := cf.DeleteRecords("app.example.com")
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "delete failed") {
		t.Fatalf("expected response body snippet in error, got %q", err.Error())
	}
}

func TestPowerDNSEnsureRecordsSendsIPv4AndIPv6(t *testing.T) {
	t.Parallel()

	var payload []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		payload, _ = io.ReadAll(r.Body)
		defer r.Body.Close()
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	cfg := &config.Config{ExternalDNSProvider: "powerdns", PDNSAPIURL: server.URL, PDNSAPIKey: "key", PDNSServerID: "localhost", PDNSZone: "example.com"}
	provider := kdns.NewProvider(cfg)
	pdns, ok := provider.(*kdns.PowerDNS)
	if !ok {
		t.Fatalf("expected PowerDNS provider, got %T", provider)
	}
	pdns.SetHTTPClient(server.Client())

	addrs := []netip.Addr{netip.MustParseAddr("198.51.100.5"), netip.MustParseAddr("2001:db8::5")}
	if err := pdns.EnsureRecords("app.example.com", addrs, 180); err != nil {
		t.Fatalf("ensure records: %v", err)
	}

	var body struct {
		RRsets []struct {
			Type    string `json:"type"`
			TTL     int    `json:"ttl"`
			Records []struct {
				Content  string `json:"content"`
				Disabled bool   `json:"disabled"`
			} `json:"records"`
		} `json:"rrsets"`
	}
	if err := json.Unmarshal(payload, &body); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}
	if len(body.RRsets) != 2 {
		t.Fatalf("expected two rrsets, got %#v", body.RRsets)
	}
	if body.RRsets[0].Type != "A" || body.RRsets[0].TTL != 180 || body.RRsets[0].Records[0].Content != "198.51.100.5" {
		t.Fatalf("unexpected A rrset: %#v", body.RRsets[0])
	}
	if body.RRsets[1].Type != "AAAA" || body.RRsets[1].TTL != 180 || body.RRsets[1].Records[0].Content != "2001:db8::5" {
		t.Fatalf("unexpected AAAA rrset: %#v", body.RRsets[1])
	}
}

func TestPowerDNSDeleteRecordsRemovesBothFamilies(t *testing.T) {
	t.Parallel()

	var payload []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("expected PATCH, got %s", r.Method)
		}
		payload, _ = io.ReadAll(r.Body)
		defer r.Body.Close()
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)

	cfg := &config.Config{ExternalDNSProvider: "powerdns", PDNSAPIURL: server.URL, PDNSAPIKey: "key", PDNSServerID: "localhost", PDNSZone: "example.com"}
	provider := kdns.NewProvider(cfg)
	pdns, ok := provider.(*kdns.PowerDNS)
	if !ok {
		t.Fatalf("expected PowerDNS provider, got %T", provider)
	}
	pdns.SetHTTPClient(server.Client())

	if err := pdns.DeleteRecords("app.example.com"); err != nil {
		t.Fatalf("delete records: %v", err)
	}

	if !strings.Contains(string(payload), "\"type\":\"A\"") || !strings.Contains(string(payload), "\"type\":\"AAAA\"") {
		t.Fatalf("expected delete payload to include A and AAAA rrsets: %s", string(payload))
	}
}
