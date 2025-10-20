package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (a *API) listAppReleases(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "listAppReleases")
	if !ok {
		return
	}
	projectID := chi.URLParam(r, "id")
	appID := chi.URLParam(r, "appId")
	query := r.URL.Query()
	limit := 0
	if raw := strings.TrimSpace(query.Get("limit")); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid limit"})
			return
		}
		limit = n
	}
	cursor := strings.TrimSpace(query.Get("cursor"))
	page, err := svc.ListAppReleases(r.Context(), projectID, appID, limit, cursor)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, page)
}
