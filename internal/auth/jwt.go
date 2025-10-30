package auth

import (
    "crypto/hmac"
    "crypto/sha256"
    "encoding/base64"
    "encoding/json"
    "errors"
    "strings"
    "time"
)

type Claims struct {
    Iss string `json:"iss,omitempty"`
    Sub string `json:"sub,omitempty"`
    Role string `json:"role,omitempty"`
    Scope string `json:"scope,omitempty"`
    Iat int64 `json:"iat,omitempty"`
    Exp int64 `json:"exp,omitempty"`
}

func SignHS256(c *Claims, key []byte) (string, error) {
    header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
    payloadBytes, err := json.Marshal(c)
    if err != nil { return "", err }
    payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
    mac := hmac.New(sha256.New, key)
    mac.Write([]byte(header + "." + payload))
    sig := mac.Sum(nil)
    return header + "." + payload + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func VerifyHS256(token string, key []byte) (*Claims, error) {
    parts := strings.Split(token, ".")
    if len(parts) != 3 {
        return nil, errors.New("invalid token format")
    }
    mac := hmac.New(sha256.New, key)
    mac.Write([]byte(parts[0] + "." + parts[1]))
    sig, err := base64.RawURLEncoding.DecodeString(parts[2])
    if err != nil { return nil, err }
    if !hmac.Equal(sig, mac.Sum(nil)) {
        return nil, errors.New("signature mismatch")
    }
    payload, err := base64.RawURLEncoding.DecodeString(parts[1])
    if err != nil { return nil, err }
    var c Claims
    if err := json.Unmarshal(payload, &c); err != nil { return nil, err }
    if c.Exp != 0 && time.Now().Unix() > c.Exp {
        return nil, errors.New("token expired")
    }
    return &c, nil
}
