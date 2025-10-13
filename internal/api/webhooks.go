package api

import (
	"encoding/json"
	"io"
	"net/http"
)

// POST /v1/webhooks/git
// Accepts generic push payloads. Signature verification (per-app secret and/or global) is handled by the service.
func (a *API) gitWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	svc, ok := a.serviceOrError(w, "gitWebhook")
	if !ok {
		return
	}
	sig := r.Header.Get("X-Hub-Signature-256")
	if err := svc.HandleGitWebhook(r.Context(), payload, body, sig); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "handled"})
}
