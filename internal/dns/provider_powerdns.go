package dns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
)

type PowerDNS struct {
	apiURL   string
	apiKey   string
	serverID string
	zone     string
	client   *http.Client
}

func (p *PowerDNS) EnsureRecords(host string, addrs []netip.Addr, ttl int) error {
	if host == "" {
		return nil
	}
	ipv4, ipv6 := splitAddrs(addrs)
	if len(ipv4) == 0 && len(ipv6) == 0 {
		return fmt.Errorf("no valid ip addresses for %s", host)
	}
	name := ensureTrailingDot(host)
	zone := ensureTrailingDot(p.zone)
	endpoint := fmt.Sprintf("%s/api/v1/servers/%s/zones/%s", strings.TrimRight(p.apiURL, "/"), url.PathEscape(p.serverID), url.PathEscape(zone))
	ttl = normalizeTTL(ttl)

	rrsets := make([]any, 0, 2)
	if len(ipv4) > 0 {
		rrsets = append(rrsets, map[string]any{
			"name":       name,
			"type":       "A",
			"ttl":        ttl,
			"changetype": "REPLACE",
			"records":    makePowerDNSRecords(ipv4),
		})
	}
	if len(ipv6) > 0 {
		rrsets = append(rrsets, map[string]any{
			"name":       name,
			"type":       "AAAA",
			"ttl":        ttl,
			"changetype": "REPLACE",
			"records":    makePowerDNSRecords(ipv6),
		})
	}

	body := map[string]any{"rrsets": rrsets}
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("powerdns marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPatch, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("powerdns request: %w", err)
	}
	req.Header.Set("X-API-Key", p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("powerdns update records: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("powerdns update failed: %s", resp.Status)
	}
	return nil
}

func (p *PowerDNS) DeleteRecords(host string) error {
	if host == "" {
		return nil
	}
	name := ensureTrailingDot(host)
	zone := ensureTrailingDot(p.zone)
	endpoint := fmt.Sprintf("%s/api/v1/servers/%s/zones/%s", strings.TrimRight(p.apiURL, "/"), url.PathEscape(p.serverID), url.PathEscape(zone))
	body := map[string]any{
		"rrsets": []any{
			map[string]any{"name": name, "type": "A", "changetype": "DELETE"},
			map[string]any{"name": name, "type": "AAAA", "changetype": "DELETE"},
		},
	}
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("powerdns marshal request: %w", err)
	}
	req, err := http.NewRequest(http.MethodPatch, endpoint, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("powerdns request: %w", err)
	}
	req.Header.Set("X-API-Key", p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.httpClient().Do(req)
	if err != nil {
		return fmt.Errorf("powerdns delete records: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("powerdns delete failed: %s", resp.Status)
	}
	return nil
}

func (p *PowerDNS) httpClient() *http.Client {
	if p.client != nil {
		return p.client
	}
	return &http.Client{Timeout: defaultHTTPTimeout}
}

func (p *PowerDNS) SetHTTPClient(client *http.Client) {
	if client != nil {
		p.client = client
	}
}

func makePowerDNSRecords(addrs []netip.Addr) []any {
	records := make([]any, 0, len(addrs))
	for _, addr := range addrs {
		records = append(records, map[string]any{"content": addr.String(), "disabled": false})
	}
	return records
}

func ensureTrailingDot(s string) string {
	if strings.HasSuffix(s, ".") {
		return s
	}
	return s + "."
}
