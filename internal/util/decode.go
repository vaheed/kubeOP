package util

import (
	"encoding/base64"
	"errors"
	"strings"
)

// DecodeKubeconfig returns a plaintext kubeconfig from a base64-encoded input.
// Policy: kubeconfig_b64 is required; plaintext kubeconfig is not accepted.
func DecodeKubeconfig(_ string, kubeconfigB64 string) (string, error) {
	b64 := strings.TrimSpace(kubeconfigB64)
	if b64 == "" {
		return "", errors.New("kubeconfig_b64 is required")
	}
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", errors.New("invalid base64")
	}
	return string(b), nil
}
