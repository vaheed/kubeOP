package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
)

// DeriveKey normalizes a provided key string into a 32-byte key suitable for AES-256.
// Accepts base64 or hex strings; otherwise SHA-256 of raw bytes.
func DeriveKey(s string) []byte {
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		if len(b) >= 32 {
			return b[:32]
		}
		h := sha256.Sum256(b)
		return h[:]
	}
	if b, err := hex.DecodeString(s); err == nil {
		if len(b) >= 32 {
			return b[:32]
		}
		h := sha256.Sum256(b)
		return h[:]
	}
	h := sha256.Sum256([]byte(s))
	return h[:]
}

// EncryptAESGCM encrypts plaintext using AES-GCM. Returns nonce||ciphertext.
func EncryptAESGCM(plaintext, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	sealed := gcm.Seal(nil, nonce, plaintext, []byte("kcfg-v1"))
	out := make([]byte, len(nonce)+len(sealed))
	copy(out, nonce)
	copy(out[len(nonce):], sealed)
	return out, nil
}

// DecryptAESGCM decrypts combined nonce||ciphertext using AES-GCM.
func DecryptAESGCM(data, key []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	n := gcm.NonceSize()
	if len(data) < n+gcm.Overhead() {
		return nil, errors.New("ciphertext too short")
	}
	nonce := data[:n]
	ct := data[n:]
	pt, err := gcm.Open(nil, nonce, ct, []byte("kcfg-v1"))
	if err != nil {
		return nil, err
	}
	return pt, nil
}
