package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

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

func writeMaintenanceError(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	var me service.MaintenanceError
	msg := service.ErrMaintenanceEnabled.Error()
	if errors.As(err, &me) {
		trimmed := strings.TrimSpace(me.Message)
		if trimmed != "" {
			msg = fmt.Sprintf("%s: %s", msg, trimmed)
		}
	} else if !errors.Is(err, service.ErrMaintenanceEnabled) {
		return false
	}
	writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": msg})
	return true
}
