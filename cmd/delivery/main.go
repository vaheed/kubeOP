package main

import (
    "encoding/json"
    "log"
    "net/http"
    "os"

    api "github.com/vaheed/kubeop/internal/api"
    "github.com/vaheed/kubeop/internal/version"
)

func main() {
    addr := os.Getenv("DELIVERY_HTTP_ADDR")
    if addr == "" { addr = ":8091" }
    mux := http.NewServeMux()
    mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
    mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
    mux.HandleFunc("/version", func(w http.ResponseWriter, _ *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]string{
            "service":   "delivery",
            "version":   version.Version,
            "gitCommit": version.Build,
            "buildDate": version.BuildDate,
        })
    })
    mux.Handle("/metrics", api.PromHandler())
    log.Printf("delivery listening on %s", addr)
    log.Fatal(http.ListenAndServe(addr, mux))
}

