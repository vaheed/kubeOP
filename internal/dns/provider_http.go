package dns

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

const defaultHTTPTimeout = 10 * time.Second

type HTTPProvider struct {
	baseURL string
	apiKey  string
	client  *http.Client
}

func newHTTPProvider(baseURL, apiKey string, client *http.Client) *HTTPProvider {
	trimmed := strings.TrimRight(baseURL, "/")
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return &HTTPProvider{baseURL: trimmed, apiKey: apiKey, client: client}
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
	payload := ensurePayload{FQDN: host, TTL: normalizeTTL(ttl)}
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
	return &http.Client{Timeout: defaultHTTPTimeout}
}

func (p *HTTPProvider) SetHTTPClient(client *http.Client) {
	if client != nil {
		p.client = client
	}
}
