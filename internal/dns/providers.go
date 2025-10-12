package dns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"kubeop/internal/config"
)

type Provider interface {
	EnsureARecord(host, ip string, ttl int) error
	DeleteARecord(host string) error
}

func NewProvider(cfg *config.Config) Provider {
	switch strings.ToLower(cfg.ExternalDNSProvider) {
	case "cloudflare":
		if cfg.CFAPIToken != "" && cfg.CFZoneID != "" {
			return &Cloudflare{token: cfg.CFAPIToken, zoneID: cfg.CFZoneID}
		}
	case "powerdns":
		zone := cfg.PDNSZone
		if zone == "" {
			zone = cfg.PaaSDomain
		}
		if cfg.PDNSAPIURL != "" && cfg.PDNSAPIKey != "" && zone != "" {
			server := cfg.PDNSServerID
			if server == "" {
				server = "localhost"
			}
			return &PowerDNS{apiURL: strings.TrimRight(cfg.PDNSAPIURL, "/"), apiKey: cfg.PDNSAPIKey, serverID: server, zone: zone}
		}
	}
	return nil
}

// ---------------- Cloudflare ----------------

type Cloudflare struct{ token, zoneID string }

func (c *Cloudflare) EnsureARecord(host, ip string, ttl int) error {
	base := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", url.PathEscape(c.zoneID))
	client := &http.Client{Timeout: 10 * time.Second}
	// Find existing
	req, _ := http.NewRequest(http.MethodGet, base+"?type=A&name="+url.QueryEscape(host), nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	by, _ := io.ReadAll(resp.Body)
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("cloudflare list failed: %s", resp.Status)
	}
	var lst struct {
		Result []struct {
			ID      string `json:"id"`
			Content string `json:"content"`
		} `json:"result"`
	}
	_ = json.Unmarshal(by, &lst)
	if len(lst.Result) > 0 {
		id := lst.Result[0].ID
		// Update if different
		body := map[string]any{"type": "A", "name": host, "content": ip, "ttl": ttl, "proxied": false}
		b, _ := json.Marshal(body)
		req, _ = http.NewRequest(http.MethodPut, base+"/"+url.PathEscape(id), bytes.NewReader(b))
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Content-Type", "application/json")
		resp2, err := client.Do(req)
		if err != nil {
			return err
		}
		defer resp2.Body.Close()
		if resp2.StatusCode/100 != 2 {
			return fmt.Errorf("cloudflare update failed: %s", resp2.Status)
		}
		return nil
	}
	// Create
	body := map[string]any{"type": "A", "name": host, "content": ip, "ttl": ttl, "proxied": false}
	b, _ := json.Marshal(body)
	req, _ = http.NewRequest(http.MethodPost, base, bytes.NewReader(b))
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp3, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp3.Body.Close()
	if resp3.StatusCode/100 != 2 {
		return fmt.Errorf("cloudflare create failed: %s", resp3.Status)
	}
	return nil
}

func (c *Cloudflare) DeleteARecord(host string) error {
	base := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records", url.PathEscape(c.zoneID))
	client := &http.Client{Timeout: 10 * time.Second}
	// Find existing
	req, _ := http.NewRequest(http.MethodGet, base+"?type=A&name="+url.QueryEscape(host), nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("cloudflare list failed: %s", resp.Status)
	}
	var lst struct {
		Result []struct {
			ID string `json:"id"`
		} `json:"result"`
	}
	by, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(by, &lst)
	if len(lst.Result) == 0 {
		return nil
	}
	id := lst.Result[0].ID
	// Delete
	req, _ = http.NewRequest(http.MethodDelete, base+"/"+url.PathEscape(id), nil)
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	resp2, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp2.Body.Close()
	if resp2.StatusCode/100 != 2 {
		return fmt.Errorf("cloudflare delete failed: %s", resp2.Status)
	}
	return nil
}

// ---------------- PowerDNS ----------------

type PowerDNS struct {
	apiURL   string
	apiKey   string
	serverID string
	zone     string
}

func (p *PowerDNS) EnsureARecord(host, ip string, ttl int) error {
	// host may be FQDN; PDNS expects trailing dot on names
	name := host
	if !strings.HasSuffix(name, ".") {
		name += "."
	}
	zone := p.zone
	if !strings.HasSuffix(zone, ".") {
		zone += "."
	}
	endpoint := fmt.Sprintf("%s/api/v1/servers/%s/zones/%s", strings.TrimRight(p.apiURL, "/"), url.PathEscape(p.serverID), url.PathEscape(zone))
	body := map[string]any{
		"rrsets": []any{map[string]any{
			"name":       name,
			"type":       "A",
			"ttl":        ttl,
			"changetype": "REPLACE",
			"records":    []any{map[string]any{"content": ip, "disabled": false}},
		}},
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPatch, endpoint, bytes.NewReader(b))
	req.Header.Set("X-API-Key", p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("powerdns update failed: %s", resp.Status)
	}
	return nil
}

func (p *PowerDNS) DeleteARecord(host string) error {
	name := host
	if !strings.HasSuffix(name, ".") {
		name += "."
	}
	zone := p.zone
	if !strings.HasSuffix(zone, ".") {
		zone += "."
	}
	endpoint := fmt.Sprintf("%s/api/v1/servers/%s/zones/%s", strings.TrimRight(p.apiURL, "/"), url.PathEscape(p.serverID), url.PathEscape(zone))
	body := map[string]any{
		"rrsets": []any{map[string]any{
			"name":       name,
			"type":       "A",
			"changetype": "DELETE",
		}},
	}
	b, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPatch, endpoint, bytes.NewReader(b))
	req.Header.Set("X-API-Key", p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("powerdns delete failed: %s", resp.Status)
	}
	return nil
}
