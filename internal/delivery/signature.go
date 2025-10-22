package delivery

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// VerifyHMACSHA256 checks the provided signature against the payload using the shared secret.
// It accepts GitHub-style headers in the form "sha256=<hex>" or raw hex digests.
func VerifyHMACSHA256(body []byte, secret, signature string) bool {
	secret = strings.TrimSpace(secret)
	if len(body) == 0 || secret == "" {
		return false
	}
	signature = strings.TrimSpace(signature)
	if signature == "" {
		return false
	}
	sig := signature
	if strings.HasPrefix(strings.ToLower(sig), "sha256=") {
		sig = sig[len("sha256="):]
	}
	expected, err := hex.DecodeString(sig)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	actual := mac.Sum(nil)
	return hmac.Equal(actual, expected)
}
