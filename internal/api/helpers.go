package api

import (
	"context"
	"net/http"

	"go.uber.org/zap"
	httpmw "kubeop/internal/http/middleware"
	"kubeop/internal/logging"
	"kubeop/internal/service"
)

func (a *API) serviceOrError(w http.ResponseWriter, endpoint string) (*service.Service, bool) {
	if a == nil || a.svc == nil {
		logging.L().Warn("api service unavailable", zap.String("endpoint", endpoint))
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "service unavailable"})
		return nil, false
	}
	return a.svc, true
}

func contextWithActor(r *http.Request) context.Context {
	return service.ContextWithActor(r.Context(), httpmw.UserIDFromContext(r.Context()))
}
