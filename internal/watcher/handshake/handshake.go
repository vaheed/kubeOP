package handshake

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// Perform executes the watcher handshake against the kubeOP API and returns
// the cluster identifier confirmed by the server. The request uses the provided
// HTTP client when not nil; otherwise a client with the supplied timeout is
// used.
func Perform(ctx context.Context, client *http.Client, url, token, expectedCluster string) (string, error) {
	if client == nil {
		client = &http.Client{}
	}
	var requestBody io.Reader
	trimmedExpected := strings.TrimSpace(expectedCluster)
	if trimmedExpected != "" {
		payload, err := json.Marshal(map[string]string{"cluster_id": trimmedExpected})
		if err != nil {
			return "", fmt.Errorf("encode handshake payload: %w", err)
		}
		requestBody = bytes.NewReader(payload)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, requestBody)
	if err != nil {
		return "", fmt.Errorf("build handshake request: %w", err)
	}
	if trimmedExpected != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("handshake request: %w", err)
	}
	defer resp.Body.Close()
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("read handshake response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		trimmed := strings.TrimSpace(string(bodyBytes))
		return "", fmt.Errorf("handshake unexpected status %d: %s", resp.StatusCode, trimmed)
	}
	var payload struct {
		Status    string `json:"status"`
		ClusterID string `json:"cluster_id"`
	}
	if len(bodyBytes) > 0 {
		if err := json.Unmarshal(bodyBytes, &payload); err != nil {
			return "", fmt.Errorf("decode handshake response: %w", err)
		}
	}
	if payload.ClusterID == "" {
		payload.ClusterID = expectedCluster
	}
	if payload.ClusterID == "" {
		return "", errors.New("handshake response missing cluster_id")
	}
	if expectedCluster != "" && payload.ClusterID != expectedCluster {
		return "", fmt.Errorf("handshake cluster mismatch: expected %s got %s", expectedCluster, payload.ClusterID)
	}
	return payload.ClusterID, nil
}
