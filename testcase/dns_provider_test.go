package testcase

import (
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

