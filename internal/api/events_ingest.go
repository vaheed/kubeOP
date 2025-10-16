package api

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"go.uber.org/zap"
	"kubeop/internal/logging"
	"kubeop/internal/sink"
)

const maxWatcherPayload = 2 * 1024 * 1024 // 2 MiB safety limit

func (a *API) ingestWatcherEvents(w http.ResponseWriter, r *http.Request) {
	if a == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "api unavailable"})
		return
	}
	if a.cfg == nil || !a.cfg.K8SEventsBridge {
		logging.L().Debug("watcher_events_ingest_skipped", zap.String("reason", "bridge_disabled"))
		writeJSON(w, http.StatusAccepted, map[string]any{
			"status":    "ignored",
			"accepted":  0,
			"dropped":   0,
			"total":     0,
			"clusterId": "",
		})
		return
	}

	svc, ok := a.serviceOrError(w, "ingestWatcherEvents")
	if !ok {
		return
	}
	claims := claimsFromContext(r.Context())
	clusterID := strings.TrimSpace(clusterIDFromClaims(claims))
	if clusterID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "cluster id missing"})
		return
	}

	events, err := decodeWatcherEvents(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	res, svcErr := svc.ProcessWatcherEvents(r.Context(), clusterID, events)
	if svcErr != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": svcErr.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, res)
}

func decodeWatcherEvents(r *http.Request) ([]sink.Event, error) {
	if r.Body == nil {
		return nil, errors.New("empty body")
	}
	defer r.Body.Close()

	reader := io.LimitReader(r.Body, maxWatcherPayload)
	enc := strings.ToLower(strings.TrimSpace(r.Header.Get("Content-Encoding")))
	if enc == "gzip" {
		gz, err := gzip.NewReader(reader)
		if err != nil {
			return nil, fmt.Errorf("decode gzip: %w", err)
		}
		defer gz.Close()
		reader = io.LimitReader(gz, maxWatcherPayload*2)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}
	if len(body) == 0 {
		return nil, nil
	}
	var events []sink.Event
	if err := json.Unmarshal(body, &events); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}
	return events, nil
}
