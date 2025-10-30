package main

import (
    "encoding/json"
    "log"
    "net/http"
)

type DNSRecord struct {
    Host   string `json:"host"`
    Target string `json:"target"`
}

func main() {
    mux := http.NewServeMux()
    mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
    mux.HandleFunc("/v1/dnsrecords", func(w http.ResponseWriter, r *http.Request) {
        if r.Method != http.MethodPost { http.Error(w, "method", http.StatusMethodNotAllowed); return }
        var rec DNSRecord
        _ = json.NewDecoder(r.Body).Decode(&rec)
        json.NewEncoder(w).Encode(map[string]any{"status": "ok", "record": rec})
    })
    log.Println("dns-mock listening on :8080")
    log.Fatal(http.ListenAndServe(":8080", mux))
}

