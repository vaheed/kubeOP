package kms

import (
    "crypto/rand"
    "encoding/base64"
    "testing"
)

func TestEncryptDecrypt(t *testing.T) {
    key := make([]byte, 32)
    if _, err := rand.Read(key); err != nil { t.Fatal(err) }
    e, err := New(key)
    if err != nil { t.Fatal(err) }
    msg := []byte("hello world")
    ct, err := e.Encrypt(msg)
    if err != nil { t.Fatal(err) }
    pt, err := e.Decrypt(ct)
    if err != nil { t.Fatal(err) }
    if string(pt) != string(msg) {
        t.Fatalf("roundtrip mismatch: got=%q ct=%s", pt, base64.StdEncoding.EncodeToString(ct))
    }
}

