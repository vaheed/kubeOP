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
)

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
		client:     &http.Client{Timeout: defaultHTTPTimeout},
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
	return &http.Client{Timeout: defaultHTTPTimeout}
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
