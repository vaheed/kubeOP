package testcase

import (
    "testing"
    "kubeop/internal/crypto"
)

func TestDeriveKey_LengthAndVariants(t *testing.T) {
    cases := []string{
        // base64 (32 bytes: "abcdefghijklmnopqrstuvwxyz012345")
        "YWJjZGVmZ2hpamtsbW5vcHFyc3R1dnd4eTAxMjM0NQ==",
        // hex (32 bytes of 0xaa)
        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
        // raw string
        "some-random-key-value",
    }
    for _, in := range cases {
        got := crypto.DeriveKey(in)
        if len(got) != 32 {
            t.Fatalf("expected 32-byte key, got %d", len(got))
        }
    }
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
    key := crypto.DeriveKey("test-key-123")
    plaintext := []byte("hello, kubeop!")

    ct, err := crypto.EncryptAESGCM(plaintext, key)
    if err != nil {
        t.Fatalf("encrypt error: %v", err)
    }
    if string(ct) == string(plaintext) {
        t.Fatalf("ciphertext must differ from plaintext")
    }

    pt, err := crypto.DecryptAESGCM(ct, key)
    if err != nil {
        t.Fatalf("decrypt error: %v", err)
    }
    if string(pt) != string(plaintext) {
        t.Fatalf("roundtrip mismatch: got %q want %q", string(pt), string(plaintext))
    }
}

func TestDecrypt_ErrOnShortCiphertext(t *testing.T) {
    key := crypto.DeriveKey("another-key")
    _, err := crypto.DecryptAESGCM([]byte("short"), key)
    if err == nil {
        t.Fatalf("expected error for short ciphertext")
    }
}

