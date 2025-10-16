package dns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

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
		return &HTTPProvider{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, client: &http.Client{Timeout: 10 * time.Second}}
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

type HTTPProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

type ensurePayload struct {
	FQDN    string          `json:"fqdn"`
	TTL     int             `json:"ttl"`
	Records []recordPayload `json:"records"`
}

type recordPayload struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type apiError struct {
	Error string `json:"error"`
}

func (p *HTTPProvider) EnsureRecords(host string, addrs []netip.Addr, ttl int) error {
	if host == "" || len(addrs) == 0 {
		return nil
	}
	payload := ensurePayload{FQDN: host, TTL: ttl}
	payload.Records = make([]recordPayload, 0, len(addrs))
	for _, addr := range addrs {
		if !addr.IsValid() {
			continue
		}
		recType := "A"
		if addr.Is6() {
			recType = "AAAA"
		}
		payload.Records = append(payload.Records, recordPayload{Type: recType, Value: addr.String()})
	}
	if len(payload.Records) == 0 {
		return fmt.Errorf("no valid ip addresses for %s", host)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	endpoint, err := url.JoinPath(p.baseURL, "records")
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	resp, err := p.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr apiError
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error != "" {
			return fmt.Errorf("dns api ensure failed: %s (%s)", resp.Status, apiErr.Error)
		}
		return fmt.Errorf("dns api ensure failed: %s", resp.Status)
	}
	return nil
}

func (p *HTTPProvider) DeleteRecords(host string) error {
	if host == "" {
		return nil
	}
	endpoint, err := url.Parse(p.baseURL)
	if err != nil {
		return err
	}
	endpoint.Path = strings.TrimRight(endpoint.Path, "/") + "/records"
	query := endpoint.Query()
	query.Set("fqdn", host)
	endpoint.RawQuery = query.Encode()
	req, err := http.NewRequest(http.MethodDelete, endpoint.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	resp, err := p.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr apiError
		_ = json.NewDecoder(resp.Body).Decode(&apiErr)
		if apiErr.Error != "" {
			return fmt.Errorf("dns api delete failed: %s (%s)", resp.Status, apiErr.Error)
		}
		return fmt.Errorf("dns api delete failed: %s", resp.Status)
	}
	return nil
}

func (p *HTTPProvider) httpClient() *http.Client {
	if p.client != nil {
		return p.client
	}
	return &http.Client{Timeout: 10 * time.Second}
}

func (p *HTTPProvider) SetHTTPClient(client *http.Client) {
	if client != nil {
		p.client = client
	}
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

func (c *Cloudflare) EnsureRecords(host string, addrs []netip.Addr, ttl int) error {
	if host == "" {
		return nil
	}
	ipv4, ipv6 := splitAddrs(addrs)
	if len(ipv4) == 0 && len(ipv6) == 0 {
		return fmt.Errorf("no valid ip addresses for %s", host)
	}
	if len(ipv4) > 0 {
		if err := c.syncRecords(host, "A", ipv4, ttl); err != nil {
			return err
		}
	}
	if len(ipv6) > 0 {
		if err := c.syncRecords(host, "AAAA", ipv6, ttl); err != nil {
			return err
		}
	}
	return nil
}

func (c *Cloudflare) syncRecords(host, recordType string, addrs []netip.Addr, ttl int) error {
	records, err := c.listRecords(host, recordType)
	if err != nil {
		return err
	}
	endpoint := c.recordsEndpoint()
	client := c.httpClient()
	ttl = normalizeTTL(ttl)

	for i, addr := range addrs {
		payload := map[string]any{
			"type":    recordType,
			"name":    host,
			"content": addr.String(),
			"ttl":     ttl,
			"proxied": false,
		}
		if i < len(records) {
			req, err := c.newRequest(http.MethodPut, endpoint+"/"+url.PathEscape(records[i].ID), payload)
			if err != nil {
				return fmt.Errorf("cloudflare update %s record: %w", recordType, err)
			}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("cloudflare update %s record: %w", recordType, err)
			}
			var out struct{}
			if err := decodeCloudflareResponse(resp, &out); err != nil {
				return err
			}
			continue
		}
		req, err := c.newRequest(http.MethodPost, endpoint, payload)
		if err != nil {
			return fmt.Errorf("cloudflare create %s record: %w", recordType, err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("cloudflare create %s record: %w", recordType, err)
		}
		var out struct{}
		if err := decodeCloudflareResponse(resp, &out); err != nil {
			return err
		}
	}

	if len(records) > len(addrs) {
		for _, rec := range records[len(addrs):] {
			req, err := c.newRequest(http.MethodDelete, endpoint+"/"+url.PathEscape(rec.ID), nil)
			if err != nil {
				return fmt.Errorf("cloudflare delete %s record: %w", recordType, err)
			}
			resp, err := client.Do(req)
			if err != nil {
				return fmt.Errorf("cloudflare delete %s record: %w", recordType, err)
			}
			var out struct{}
			if err := decodeCloudflareResponse(resp, &out); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *Cloudflare) listRecords(host, recordType string) ([]cloudflareRecord, error) {
	endpoint := c.recordsEndpoint()
	client := c.httpClient()
	req, err := c.newRequest(http.MethodGet, endpoint+"?type="+url.QueryEscape(recordType)+"&name="+url.QueryEscape(host), nil)
	if err != nil {
		return nil, fmt.Errorf("cloudflare list %s records: %w", recordType, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cloudflare list %s records: %w", recordType, err)
	}
	var records []cloudflareRecord
	if err := decodeCloudflareResponse(resp, &records); err != nil {
		return nil, err
	}
	return records, nil
}

func (c *Cloudflare) DeleteRecords(host string) error {
	if host == "" {
		return nil
	}
	if err := c.deleteTypeRecords(host, "A"); err != nil {
		return err
	}
	if err := c.deleteTypeRecords(host, "AAAA"); err != nil {
		return err
	}
	return nil
}

func (c *Cloudflare) deleteTypeRecords(host, recordType string) error {
	records, err := c.listRecords(host, recordType)
	if err != nil {
		return err
	}
	if len(records) == 0 {
		return nil
	}
	endpoint := c.recordsEndpoint()
	client := c.httpClient()
	for _, rec := range records {
		req, err := c.newRequest(http.MethodDelete, endpoint+"/"+url.PathEscape(rec.ID), nil)
		if err != nil {
			return fmt.Errorf("cloudflare delete %s record: %w", recordType, err)
		}
		resp, err := client.Do(req)
		if err != nil {
			return fmt.Errorf("cloudflare delete %s record: %w", recordType, err)
		}
		var out struct{}
		if err := decodeCloudflareResponse(resp, &out); err != nil {
			return err
		}
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
	return &http.Client{Timeout: 10 * time.Second}
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

func splitAddrs(addrs []netip.Addr) ([]netip.Addr, []netip.Addr) {
	ipv4 := make([]netip.Addr, 0, len(addrs))
	ipv6 := make([]netip.Addr, 0, len(addrs))
	for _, addr := range addrs {
		if !addr.IsValid() {
			continue
		}
		if addr.Is6() {
			ipv6 = append(ipv6, addr)
		} else {
			ipv4 = append(ipv4, addr)
		}
	}
	return ipv4, ipv6
}

func normalizeTTL(ttl int) int {
	if ttl <= 0 {
		return 300
	}
	return ttl
}
