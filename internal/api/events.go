package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"kubeop/internal/service"
	"kubeop/internal/store"
)

func (a *API) listProjectEvents(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "listProjectEvents")
	if !ok {
		return
	}
	projectID := chi.URLParam(r, "id")
	q := r.URL.Query()
	filter := store.ProjectEventFilter{}
	if kinds := parseCSV(q.Get("kind")); len(kinds) > 0 {
		filter.Kinds = kinds
	}
	if severities := parseCSV(q.Get("severity")); len(severities) > 0 {
		filter.Severities = severities
	}
	filter.ActorUserID = strings.TrimSpace(q.Get("actor"))
	if sinceStr := strings.TrimSpace(q.Get("since")); sinceStr != "" {
		if ts, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			filter.Since = ts
		}
	}
	if limitStr := strings.TrimSpace(q.Get("limit")); limitStr != "" {
		if lim, err := strconv.Atoi(limitStr); err == nil {
			filter.Limit = lim
		}
	}
	if cursor := strings.TrimSpace(q.Get("cursor")); cursor != "" {
		filter.Cursor = cursor
	}
	filter.Search = strings.TrimSpace(q.Get("grep"))
	if filter.Search == "" {
		filter.Search = strings.TrimSpace(q.Get("search"))
	}
	page, err := svc.ListProjectEvents(r.Context(), projectID, filter)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, page)
}

type customEventRequest struct {
	Kind     string         `json:"kind"`
	Severity string         `json:"severity"`
	Message  string         `json:"message"`
	AppID    string         `json:"appId"`
	Meta     map[string]any `json:"meta"`
}

func (a *API) appendProjectEvent(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "appendProjectEvent")
	if !ok {
		return
	}
	projectID := chi.URLParam(r, "id")
	var req customEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	ctx := contextWithActor(r)
	evt, err := svc.AppendProjectEvent(ctx, service.EventInput{
		ProjectID: projectID,
		AppID:     req.AppID,
		Kind:      req.Kind,
		Severity:  req.Severity,
		Message:   req.Message,
		Meta:      req.Meta,
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, evt)
}

func parseCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
