package main

import (
    "crypto/rand"
    "encoding/base64"
    "encoding/json"
    "log"
    "net/http"
)

type CertReq struct {
    Host string `json:"host"`
}

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
    mux.HandleFunc("/v1/certificates", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost { http.Error(w, "method", http.StatusMethodNotAllowed); return }
        var req CertReq
        _ = json.NewDecoder(r.Body).Decode(&req)
        der := make([]byte, 256)
        rand.Read(der)
        pem := base64.StdEncoding.EncodeToString(der)
        json.NewEncoder(w).Encode(map[string]any{"status": "ok", "host": req.Host, "certificate": pem, "private_key": pem})
    })
    log.Println("acme-mock listening on :8080")
    log.Fatal(http.ListenAndServe(":8080", mux))
}
