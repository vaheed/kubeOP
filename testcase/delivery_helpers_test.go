package testcase

import (
	"testing"

	"kubeop/internal/delivery"
)

func TestVerifyHMACSHA256(t *testing.T) {
	payload := []byte("hello")
	secret := "topsecret"
	mac := delivery.VerifyHMACSHA256(payload, secret, "sha256=5d41402abc4b2a76b9719d911017c592")
	if mac {
		t.Fatalf("expected mismatch due to different digest")
	}
	mac = delivery.VerifyHMACSHA256(payload, secret, "sha256="+"\n")
	if mac {
		t.Fatalf("expected mismatch for invalid hex")
	}
	hmac := delivery.VerifyHMACSHA256(payload, secret, "sha256=ed76fd36523b8becda5a3b36d0e3737e8ae5111f55e26c7c3a455a3ce29636d2")
	if !hmac {
		t.Fatalf("expected signature to validate")
	}
}

func TestBuildSBOM(t *testing.T) {
	manifests := []string{"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: demo\n"}
	sbom := delivery.BuildSBOM("manifests", manifests, map[string]string{"gitCommit": "abc123"})
	if sbom["sourceType"] != "manifests" {
		t.Fatalf("expected sourceType manifests, got %#v", sbom["sourceType"])
	}
	docs, ok := sbom["manifests"].([]map[string]string)
	if !ok || len(docs) != 1 {
		t.Fatalf("expected one manifest digest, got %#v", sbom["manifests"])
	}
	if docs[0]["sha256"] == "" {
		t.Fatalf("expected sha256 digest to be populated")
	}
	if sbom["gitCommit"] != "abc123" {
		t.Fatalf("expected gitCommit metadata")
	}
}
