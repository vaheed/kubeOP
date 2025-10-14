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
			return NewCloudflare(cfg.CFAPIToken, cfg.CFZoneID)
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

type Cloudflare struct {
	token      string
	zoneID     string
	client     *http.Client
	apiBaseURL string
}

func NewCloudflare(token, zoneID string) *Cloudflare {
	return &Cloudflare{
		token:      token,
		zoneID:     zoneID,
		client:     &http.Client{Timeout: 10 * time.Second},
		apiBaseURL: "https://api.cloudflare.com/client/v4",
	}
}

func (c *Cloudflare) SetHTTPClient(client *http.Client) {
	if client != nil {
		c.client = client
	}
}

func (c *Cloudflare) SetAPIBaseURL(base string) {
	if strings.TrimSpace(base) == "" {
		return
	}
	c.apiBaseURL = strings.TrimRight(base, "/")
}

func (c *Cloudflare) httpClient() *http.Client {
	if c.client != nil {
		return c.client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (c *Cloudflare) recordsEndpoint() string {
	base := strings.TrimRight(c.apiBaseURL, "/")
	return fmt.Sprintf("%s/zones/%s/dns_records", base, url.PathEscape(c.zoneID))
}

func (c *Cloudflare) newRequest(method, endpoint string, payload any) (*http.Request, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("marshal cloudflare payload: %w", err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, endpoint, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

func (c *Cloudflare) EnsureARecord(host, ip string, ttl int) error {
	endpoint := c.recordsEndpoint()
	client := c.httpClient()

	req, err := c.newRequest(http.MethodGet, endpoint+"?type=A&name="+url.QueryEscape(host), nil)
	if err != nil {
		return fmt.Errorf("cloudflare list records: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare list records: %w", err)
	}
	var records []cloudflareRecord
	if err := decodeCloudflareResponse(resp, &records); err != nil {
		return fmt.Errorf("cloudflare list records: %w", err)
	}

	payload := map[string]any{"type": "A", "name": host, "content": ip, "ttl": ttl, "proxied": false}
	if len(records) > 0 {
		req, err = c.newRequest(http.MethodPut, endpoint+"/"+url.PathEscape(records[0].ID), payload)
		if err != nil {
			return fmt.Errorf("cloudflare update record: %w", err)
		}
		resp, err = client.Do(req)
		if err != nil {
			return fmt.Errorf("cloudflare update record: %w", err)
		}
		var out struct{}
		if err := decodeCloudflareResponse(resp, &out); err != nil {
			return fmt.Errorf("cloudflare update record: %w", err)
		}
		return nil
	}

	req, err = c.newRequest(http.MethodPost, endpoint, payload)
	if err != nil {
		return fmt.Errorf("cloudflare create record: %w", err)
	}
	resp, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare create record: %w", err)
	}
	var out struct{}
	if err := decodeCloudflareResponse(resp, &out); err != nil {
		return fmt.Errorf("cloudflare create record: %w", err)
	}
	return nil
}

func (c *Cloudflare) DeleteARecord(host string) error {
	endpoint := c.recordsEndpoint()
	client := c.httpClient()

	req, err := c.newRequest(http.MethodGet, endpoint+"?type=A&name="+url.QueryEscape(host), nil)
	if err != nil {
		return fmt.Errorf("cloudflare list records: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare list records: %w", err)
	}
	var records []cloudflareRecord
	if err := decodeCloudflareResponse(resp, &records); err != nil {
		return fmt.Errorf("cloudflare list records: %w", err)
	}
	if len(records) == 0 {
		return nil
	}

	req, err = c.newRequest(http.MethodDelete, endpoint+"/"+url.PathEscape(records[0].ID), nil)
	if err != nil {
		return fmt.Errorf("cloudflare delete record: %w", err)
	}
	resp, err = client.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare delete record: %w", err)
	}
	var out struct{}
	if err := decodeCloudflareResponse(resp, &out); err != nil {
		return fmt.Errorf("cloudflare delete record: %w", err)
	}
	return nil
}

type cloudflareRecord struct {
	ID      string `json:"id"`
	Content string `json:"content"`
}

type cloudflareAPIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type cloudflareAPIMessage struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func decodeCloudflareResponse[T any](resp *http.Response, result *T) error {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response body: %w", err)
	}
	trimmed := strings.TrimSpace(string(body))
	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("%s: %s: %s", cloudflareOperation(resp), resp.Status, truncateBody(trimmed))
	}
	var envelope struct {
		Success  bool                   `json:"success"`
		Errors   []cloudflareAPIError   `json:"errors"`
		Messages []cloudflareAPIMessage `json:"messages"`
		Result   T                      `json:"result"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("%s: decode response: %w (body=%s)", cloudflareOperation(resp), err, truncateBody(trimmed))
	}
	if !envelope.Success {
		return fmt.Errorf("%s: %s", cloudflareOperation(resp), formatCloudflareErrors(envelope.Errors, envelope.Messages, truncateBody(trimmed)))
	}
	if result != nil {
		*result = envelope.Result
	}
	return nil
}

func cloudflareOperation(resp *http.Response) string {
	if resp == nil || resp.Request == nil || resp.Request.URL == nil {
		return "cloudflare request"
	}
	path := resp.Request.URL.Path
	if resp.Request.URL.RawQuery != "" {
		path += "?" + resp.Request.URL.RawQuery
	}
	return fmt.Sprintf("cloudflare %s %s", resp.Request.Method, path)
}

func formatCloudflareErrors(errors []cloudflareAPIError, messages []cloudflareAPIMessage, fallback string) string {
	parts := make([]string, 0, len(errors)+len(messages))
	for _, e := range errors {
		if e.Code != 0 {
			parts = append(parts, fmt.Sprintf("code=%d message=%s", e.Code, e.Message))
		} else {
			parts = append(parts, fmt.Sprintf("message=%s", e.Message))
		}
	}
	for _, m := range messages {
		if m.Code != 0 {
			parts = append(parts, fmt.Sprintf("code=%d message=%s", m.Code, m.Message))
		} else {
			parts = append(parts, fmt.Sprintf("message=%s", m.Message))
		}
	}
	if len(parts) == 0 {
		if fallback != "" {
			return fallback
		}
		return "unspecified error"
	}
	return strings.Join(parts, "; ")
}

func truncateBody(body string) string {
	if len(body) <= 512 {
		return body
	}
	return body[:512] + "..."
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
