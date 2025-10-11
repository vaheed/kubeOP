package api

import (
    "net/http"

    "github.com/prometheus/client_golang/prometheus/promhttp"
)

// metrics exposes Prometheus metrics. Kept outside admin auth.
func (a *API) metrics(w http.ResponseWriter, r *http.Request) {
    promhttp.Handler().ServeHTTP(w, r)
}

