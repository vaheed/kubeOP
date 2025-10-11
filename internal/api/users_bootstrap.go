package api

import (
    "encoding/json"
    "net/http"
    "kubeop/internal/service"
)

type userBootstrapReq struct {
    // Either provide an existing `userId`, or provide `name` and `email` to create (or reuse by email)
    UserID    string `json:"userId,omitempty"`
    Name      string `json:"name,omitempty"`
    Email     string `json:"email,omitempty"`
    ClusterID string `json:"clusterId"`
}

func (a *API) bootstrapUser(w http.ResponseWriter, r *http.Request) {
    var req userBootstrapReq
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        writeJSON(w, http.StatusBadRequest, map[string]string{"error":"invalid json"})
        return
    }
    out, err := a.svc.BootstrapUser(r.Context(), service.UserBootstrapInput{UserID: req.UserID, Name: req.Name, Email: req.Email, ClusterID: req.ClusterID})
    if err != nil {
        writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
        return
    }
    writeJSON(w, http.StatusCreated, out)
}

