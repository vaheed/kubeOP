package kms

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "errors"
    "io"
)

type Envelope struct { key []byte }

func New(master []byte) (*Envelope, error) {
    if len(master) != 32 { // AES-256
        return nil, errors.New("KMS master key must be 32 bytes")
    }
    return &Envelope{key: master}, nil
}

func (e *Envelope) Encrypt(plain []byte) ([]byte, error) {
    block, err := aes.NewCipher(e.key)
    if err != nil { return nil, err }
    gcm, err := cipher.NewGCM(block)
    if err != nil { return nil, err }
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return nil, err
    }
    return append(nonce, gcm.Seal(nil, nonce, plain, nil)...), nil
}

func (e *Envelope) Decrypt(ciphertext []byte) ([]byte, error) {
    block, err := aes.NewCipher(e.key)
    if err != nil { return nil, err }
    gcm, err := cipher.NewGCM(block)
    if err != nil { return nil, err }
    if len(ciphertext) < gcm.NonceSize() {
        return nil, errors.New("ciphertext too short")
    }
    nonce := ciphertext[:gcm.NonceSize()]
    data := ciphertext[gcm.NonceSize():]
    return gcm.Open(nil, nonce, data, nil)
}

