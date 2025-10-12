package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// EncodeQuotaOverrides normalises and JSON-encodes ResourceQuota overrides.
// Empty or whitespace-only keys are dropped. Returns nil when there are no overrides.
func EncodeQuotaOverrides(overrides map[string]string) ([]byte, error) {
	if len(overrides) == 0 {
		return nil, nil
	}
	cleaned := make(map[string]string, len(overrides))
	for k, v := range overrides {
		key := strings.TrimSpace(k)
		if key == "" {
			continue
		}
		cleaned[key] = strings.TrimSpace(v)
	}
	if len(cleaned) == 0 {
		return nil, nil
	}
	b, err := json.Marshal(cleaned)
	if err != nil {
		return nil, fmt.Errorf("marshal quota overrides: %w", err)
	}
	return b, nil
}

// DecodeQuotaOverrides parses overrides previously produced by EncodeQuotaOverrides.
func DecodeQuotaOverrides(raw []byte) (map[string]string, error) {
	out := map[string]string{}
	if len(bytes.TrimSpace(raw)) == 0 {
		return out, nil
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode quota overrides: %w", err)
	}
	return out, nil
}
