package util

import (
    "encoding/base64"
    "errors"
    "strings"
)

// DecodeKubeconfig returns a plaintext kubeconfig from either a raw string or base64-encoded input.
// If kubeconfigB64 is non-empty, it takes precedence. Whitespace is trimmed.
func DecodeKubeconfig(kubeconfig, kubeconfigB64 string) (string, error) {
    kc := strings.TrimSpace(kubeconfig)
    b64 := strings.TrimSpace(kubeconfigB64)
    if b64 != "" {
        b, err := base64.StdEncoding.DecodeString(b64)
        if err != nil {
            return "", errors.New("invalid base64")
        }
        return string(b), nil
    }
    return kc, nil
}

