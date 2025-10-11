package testcase

import (
    "encoding/base64"
    "testing"
    "kubeop/internal/util"
)

func TestDecodeKubeconfig_PlainAndB64(t *testing.T) {
    // Plain
    got, err := util.DecodeKubeconfig("plain text", "")
    if err != nil || got != "plain text" {
        t.Fatalf("plain: got %q, err=%v", got, err)
    }
    // Base64 wins
    b64 := base64.StdEncoding.EncodeToString([]byte("from-b64"))
    got, err = util.DecodeKubeconfig("plain text", b64)
    if err != nil || got != "from-b64" {
        t.Fatalf("b64: got %q, err=%v", got, err)
    }
}

func TestDecodeKubeconfig_InvalidBase64(t *testing.T) {
    if _, err := util.DecodeKubeconfig("", "not-base64!!"); err == nil {
        t.Fatalf("expected error for invalid base64")
    }
}

