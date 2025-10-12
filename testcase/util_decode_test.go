package testcase

import (
	"encoding/base64"
	"kubeop/internal/util"
	"testing"
)

func TestDecodeKubeconfig_Base64Required(t *testing.T) {
	if _, err := util.DecodeKubeconfig("plain text", ""); err == nil {
		t.Fatalf("expected error when kubeconfig_b64 is missing")
	}
}

func TestDecodeKubeconfig_Base64OK(t *testing.T) {
	b64 := base64.StdEncoding.EncodeToString([]byte("from-b64"))
	got, err := util.DecodeKubeconfig("ignored", b64)
	if err != nil || got != "from-b64" {
		t.Fatalf("b64: got %q, err=%v", got, err)
	}
}

func TestDecodeKubeconfig_InvalidBase64(t *testing.T) {
	if _, err := util.DecodeKubeconfig("", "not-base64!!"); err == nil {
		t.Fatalf("expected error for invalid base64")
	}
}
