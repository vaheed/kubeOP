package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"kubeop/internal/service"
)

type templateCreateReq struct {
	Name             string         `json:"name"`
	Kind             string         `json:"kind"`
	Description      string         `json:"description"`
	Schema           map[string]any `json:"schema"`
	Defaults         map[string]any `json:"defaults"`
	Example          map[string]any `json:"example,omitempty"`
	Base             map[string]any `json:"base,omitempty"`
	DeliveryTemplate string         `json:"deliveryTemplate"`
}

type templateValuesReq struct {
	Values map[string]any `json:"values"`
}

func (a *API) createTemplate(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "createTemplate")
	if !ok {
		return
	}
	var req templateCreateReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	ctx := contextWithActor(r)
	out, err := svc.CreateTemplate(ctx, service.TemplateCreateInput{
		Name:             req.Name,
		Kind:             req.Kind,
		Description:      req.Description,
		Schema:           req.Schema,
		Defaults:         req.Defaults,
		Example:          req.Example,
		Base:             req.Base,
		DeliveryTemplate: req.DeliveryTemplate,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (a *API) listTemplates(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "listTemplates")
	if !ok {
		return
	}
	out, err := svc.ListTemplates(r.Context())
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) getTemplate(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "getTemplate")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	out, err := svc.GetTemplate(r.Context(), id)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) renderTemplate(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "renderTemplate")
	if !ok {
		return
	}
	id := chi.URLParam(r, "id")
	var req templateValuesReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	out, err := svc.RenderTemplate(r.Context(), id, req.Values)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *API) deployTemplate(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "deployTemplate")
	if !ok {
		return
	}
	projectID := chi.URLParam(r, "id")
	templateID := chi.URLParam(r, "templateId")
	var req templateValuesReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	ctx := contextWithActor(r)
	out, err := svc.DeployTemplate(ctx, projectID, templateID, req.Values)
	if err != nil {
		if writeMaintenanceError(w, err) {
			return
		}
		status := http.StatusBadRequest
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, out)
}
