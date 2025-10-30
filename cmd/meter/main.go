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
    addr := os.Getenv("METER_HTTP_ADDR")
    if addr == "" { addr = ":8092" }
    mux := http.NewServeMux()
    mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
    mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
    mux.HandleFunc("/version", func(w http.ResponseWriter, _ *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]string{
            "service":   "meter",
            "version":   version.Version,
            "gitCommit": version.Build,
            "buildDate": version.BuildDate,
        })
    })
    mux.Handle("/metrics", api.PromHandler())
    log.Printf("meter listening on %s", addr)
    log.Fatal(http.ListenAndServe(addr, mux))
}

