package dns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"path"
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
	provider := strings.ToLower(strings.TrimSpace(cfg.DNSProvider))
	switch provider {
	case "", "http":
		baseURL := strings.TrimSpace(cfg.DNSAPIURL)
		apiKey := strings.TrimSpace(cfg.DNSAPIKey)
		if baseURL == "" || apiKey == "" {
			return nil
		}
		return &HTTPProvider{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, client: &http.Client{Timeout: 10 * time.Second}}
	case "cloudflare":
		baseURL := strings.TrimSpace(cfg.CloudflareAPIBase)
		token := strings.TrimSpace(cfg.CloudflareAPIToken)
		zoneID := strings.TrimSpace(cfg.CloudflareZoneID)
		if baseURL == "" || token == "" || zoneID == "" {
			return nil
		}
		return &CloudflareProvider{baseURL: strings.TrimRight(baseURL, "/"), apiToken: token, zoneID: zoneID, client: &http.Client{Timeout: 15 * time.Second}}
	case "powerdns":
		baseURL := strings.TrimSpace(cfg.PowerDNSAPIURL)
		apiKey := strings.TrimSpace(cfg.PowerDNSAPIKey)
		serverID := strings.TrimSpace(cfg.PowerDNSServerID)
		zone := strings.TrimSpace(cfg.PowerDNSZone)
		if baseURL == "" || apiKey == "" || serverID == "" || zone == "" {
			return nil
		}
		return &PowerDNSProvider{baseURL: strings.TrimRight(baseURL, "/"), apiKey: apiKey, serverID: serverID, zone: ensureTrailingDot(zone), client: &http.Client{Timeout: 15 * time.Second}}
	default:
		return nil
	}
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

// -----------------------------------------------------------------------------
// Cloudflare provider implementation

type CloudflareProvider struct {
	baseURL  string
	apiToken string
	zoneID   string
	client   *http.Client
}

type cloudflareRecord struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Name    string `json:"name"`
	Content string `json:"content"`
	TTL     int    `json:"ttl"`
}

type cloudflareAPIError struct {
	Message string `json:"message"`
}

type cloudflareListResponse struct {
	Success bool                 `json:"success"`
	Errors  []cloudflareAPIError `json:"errors"`
	Result  []cloudflareRecord   `json:"result"`
}

type cloudflareMutationResponse struct {
	Success bool                 `json:"success"`
	Errors  []cloudflareAPIError `json:"errors"`
	Result  cloudflareRecord     `json:"result"`
}

func (p *CloudflareProvider) EnsureRecords(host string, addrs []netip.Addr, ttl int) error {
	if host == "" || len(addrs) == 0 {
		return nil
	}
	if ttl <= 0 {
		ttl = 300
	}
	desired := map[string]map[string]struct{}{}
	for _, addr := range addrs {
		if !addr.IsValid() {
			continue
		}
		recType := "A"
		if addr.Is6() {
			recType = "AAAA"
		}
		if _, ok := desired[recType]; !ok {
			desired[recType] = map[string]struct{}{}
		}
		desired[recType][addr.String()] = struct{}{}
	}
	if len(desired) == 0 {
		return fmt.Errorf("no valid ip addresses for %s", host)
	}
	existing, err := p.listRecords(host)
	if err != nil {
		return err
	}
	// Delete records that should no longer exist
	for _, rec := range existing {
		if rec.Name != host {
			continue
		}
		if rec.Type != "A" && rec.Type != "AAAA" {
			continue
		}
		if _, ok := desired[rec.Type][rec.Content]; !ok {
			if err := p.deleteRecord(rec.ID); err != nil {
				return err
			}
		}
	}
	// Create missing records
	for recType, values := range desired {
		for value := range values {
			if hasCloudflareRecord(existing, host, recType, value) {
				continue
			}
			if err := p.createRecord(host, recType, value, ttl); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *CloudflareProvider) DeleteRecords(host string) error {
	if host == "" {
		return nil
	}
	records, err := p.listRecords(host)
	if err != nil {
		return err
	}
	for _, rec := range records {
		if rec.Name != host {
			continue
		}
		if rec.Type != "A" && rec.Type != "AAAA" {
			continue
		}
		if err := p.deleteRecord(rec.ID); err != nil {
			return err
		}
	}
	return nil
}

func (p *CloudflareProvider) listRecords(host string) ([]cloudflareRecord, error) {
	endpoint, err := url.Parse(p.baseURL)
	if err != nil {
		return nil, err
	}
	endpoint.Path = path.Join(endpoint.Path, "zones", p.zoneID, "dns_records")
	q := endpoint.Query()
	q.Set("name", host)
	endpoint.RawQuery = q.Encode()
	req, err := http.NewRequest(http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiToken)
	resp, err := p.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cloudflare api list failed: %s", resp.Status)
	}
	var out cloudflareListResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if !out.Success {
		return nil, fmt.Errorf("cloudflare api list failed: %s", joinCloudflareErrors(out.Errors))
	}
	return out.Result, nil
}

func (p *CloudflareProvider) createRecord(host, recType, value string, ttl int) error {
	payload := map[string]any{
		"type":    recType,
		"name":    host,
		"content": value,
		"ttl":     ttl,
		"proxied": false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	endpoint, err := url.Parse(p.baseURL)
	if err != nil {
		return err
	}
	endpoint.Path = path.Join(endpoint.Path, "zones", p.zoneID, "dns_records")
	req, err := http.NewRequest(http.MethodPost, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiToken)
	resp, err := p.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("cloudflare api create failed: %s", resp.Status)
	}
	var out cloudflareMutationResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("cloudflare api create failed: %s", joinCloudflareErrors(out.Errors))
	}
	return nil
}

func (p *CloudflareProvider) deleteRecord(id string) error {
	if id == "" {
		return nil
	}
	endpoint, err := url.Parse(p.baseURL)
	if err != nil {
		return err
	}
	endpoint.Path = path.Join(endpoint.Path, "zones", p.zoneID, "dns_records", id)
	req, err := http.NewRequest(http.MethodDelete, endpoint.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+p.apiToken)
	resp, err := p.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("cloudflare api delete failed: %s", resp.Status)
	}
	var out cloudflareMutationResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return err
	}
	if !out.Success {
		return fmt.Errorf("cloudflare api delete failed: %s", joinCloudflareErrors(out.Errors))
	}
	return nil
}

func (p *CloudflareProvider) httpClient() *http.Client {
	if p.client != nil {
		return p.client
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (p *CloudflareProvider) SetHTTPClient(client *http.Client) {
	if client != nil {
		p.client = client
	}
}

func hasCloudflareRecord(records []cloudflareRecord, host, recType, value string) bool {
	for _, rec := range records {
		if rec.Name == host && rec.Type == recType && rec.Content == value {
			return true
		}
	}
	return false
}

func joinCloudflareErrors(errs []cloudflareAPIError) string {
	if len(errs) == 0 {
		return "unknown error"
	}
	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		if strings.TrimSpace(e.Message) != "" {
			msgs = append(msgs, strings.TrimSpace(e.Message))
		}
	}
	if len(msgs) == 0 {
		return "unknown error"
	}
	return strings.Join(msgs, "; ")
}

// -----------------------------------------------------------------------------
// PowerDNS provider implementation

type PowerDNSProvider struct {
	baseURL  string
	apiKey   string
	serverID string
	zone     string
	client   *http.Client
}

type powerDNSPayload struct {
	RRSets []powerDNSRRSet `json:"rrsets"`
}

type powerDNSRRSet struct {
	Name       string           `json:"name"`
	Type       string           `json:"type"`
	TTL        int              `json:"ttl,omitempty"`
	ChangeType string           `json:"changetype"`
	Records    []powerDNSRecord `json:"records,omitempty"`
}

type powerDNSRecord struct {
	Content  string `json:"content"`
	Disabled bool   `json:"disabled"`
}

func (p *PowerDNSProvider) EnsureRecords(host string, addrs []netip.Addr, ttl int) error {
	if host == "" || len(addrs) == 0 {
		return nil
	}
	if ttl <= 0 {
		ttl = 300
	}
	rrsets := make([]powerDNSRRSet, 0, 2)
	hostName := ensureTrailingDot(host)
	v4Records := make([]powerDNSRecord, 0, len(addrs))
	v6Records := make([]powerDNSRecord, 0, len(addrs))
	for _, addr := range addrs {
		if !addr.IsValid() {
			continue
		}
		rec := powerDNSRecord{Content: addr.String(), Disabled: false}
		if addr.Is6() {
			v6Records = append(v6Records, rec)
		} else {
			v4Records = append(v4Records, rec)
		}
	}
	if len(v4Records) == 0 && len(v6Records) == 0 {
		return fmt.Errorf("no valid ip addresses for %s", host)
	}
	if len(v4Records) > 0 {
		rrsets = append(rrsets, powerDNSRRSet{Name: hostName, Type: "A", TTL: ttl, ChangeType: "REPLACE", Records: v4Records})
	}
	if len(v6Records) > 0 {
		rrsets = append(rrsets, powerDNSRRSet{Name: hostName, Type: "AAAA", TTL: ttl, ChangeType: "REPLACE", Records: v6Records})
	}
	payload := powerDNSPayload{RRSets: rrsets}
	return p.patchZone(payload)
}

func (p *PowerDNSProvider) DeleteRecords(host string) error {
	if host == "" {
		return nil
	}
	hostName := ensureTrailingDot(host)
	payload := powerDNSPayload{RRSets: []powerDNSRRSet{
		{Name: hostName, Type: "A", ChangeType: "DELETE"},
		{Name: hostName, Type: "AAAA", ChangeType: "DELETE"},
	}}
	return p.patchZone(payload)
}

func (p *PowerDNSProvider) patchZone(payload powerDNSPayload) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	endpoint, err := url.Parse(p.baseURL)
	if err != nil {
		return err
	}
	endpoint.Path = path.Join(endpoint.Path, "servers", p.serverID, "zones", p.zone)
	req, err := http.NewRequest(http.MethodPatch, endpoint.String(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", p.apiKey)
	resp, err := p.httpClient().Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("powerdns api request failed: %s", resp.Status)
	}
	return nil
}

func (p *PowerDNSProvider) httpClient() *http.Client {
	if p.client != nil {
		return p.client
	}
	return &http.Client{Timeout: 15 * time.Second}
}

func (p *PowerDNSProvider) SetHTTPClient(client *http.Client) {
	if client != nil {
		p.client = client
	}
}

func ensureTrailingDot(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}
	if strings.HasSuffix(value, ".") {
		return value
	}
	return value + "."
}
