package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"kubeop/internal/service"
)

type maintenanceUpdateReq struct {
	Enabled bool   `json:"enabled"`
	Message string `json:"message,omitempty"`
}

func (a *API) getMaintenance(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "getMaintenance")
	if !ok {
		return
	}
	state, err := svc.GetMaintenanceState(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, state)
}

func (a *API) updateMaintenance(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "updateMaintenance")
	if !ok {
		return
	}
	var req maintenanceUpdateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	ctx := contextWithActor(r)
	state, err := svc.UpdateMaintenanceState(ctx, service.MaintenanceUpdateInput{Enabled: req.Enabled, Message: req.Message})
	if err != nil {
		if writeMaintenanceError(w, err) {
			return
		}
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrInvalidMaintenanceInput) {
			status = http.StatusBadRequest
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, state)
}
