package dns

import (
	"net/http"
	"net/netip"
	"strings"

	"kubeop/internal/config"
)

type Provider interface {
	EnsureRecords(host string, addrs []netip.Addr, ttl int) error
	DeleteRecords(host string) error
}

func NewProvider(cfg *config.Config) Provider {
	if cfg == nil {
		return nil
	}

	baseURL := strings.TrimSpace(cfg.DNSAPIURL)
	apiKey := strings.TrimSpace(cfg.DNSAPIKey)
	if baseURL != "" && apiKey != "" {
		return newHTTPProvider(baseURL, apiKey, &http.Client{Timeout: defaultHTTPTimeout})
	}

	switch strings.ToLower(strings.TrimSpace(cfg.ExternalDNSProvider)) {
	case "cloudflare":
		token := strings.TrimSpace(cfg.CFAPIToken)
		zone := strings.TrimSpace(cfg.CFZoneID)
		if token != "" && zone != "" {
			return NewCloudflare(token, zone)
		}
	case "powerdns":
		api := strings.TrimSpace(cfg.PDNSAPIURL)
		key := strings.TrimSpace(cfg.PDNSAPIKey)
		zone := strings.TrimSpace(cfg.PDNSZone)
		if zone == "" {
			zone = strings.TrimSpace(cfg.PaaSDomain)
		}
		if api != "" && key != "" && zone != "" {
			serverID := strings.TrimSpace(cfg.PDNSServerID)
			if serverID == "" {
				serverID = "localhost"
			}
			return &PowerDNS{apiURL: strings.TrimRight(api, "/"), apiKey: key, serverID: serverID, zone: zone}
		}
	}

	return nil
}
