package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestSummarizeWatcherClaims(t *testing.T) {
	expires := time.Now().Add(time.Hour)
	claims := jwt.MapClaims{
		"watcher_id": " watcher-1 ",
		"cluster_id": "cluster-1",
		"sub":        "watcher:watcher-1",
		"iss":        "https://api.example.com",
		"aud":        []interface{}{"events", "watchers"},
		"exp":        jwt.NewNumericDate(expires),
	}
	summary := summarizeWatcherClaims(claims)
	if summary["watcherId"].(string) != "watcher-1" {
		t.Fatalf("expected trimmed watcher id, got %#v", summary["watcherId"])
	}
	if summary["clusterId"].(string) != "cluster-1" {
		t.Fatalf("expected cluster id, got %#v", summary["clusterId"])
	}
	if summary["subject"].(string) != "watcher:watcher-1" {
		t.Fatalf("expected subject, got %#v", summary["subject"])
	}
	if summary["issuer"].(string) != "https://api.example.com" {
		t.Fatalf("expected issuer, got %#v", summary["issuer"])
	}
	audience := summary["audience"].([]string)
	if len(audience) != 2 || audience[0] != "events" || audience[1] != "watchers" {
		t.Fatalf("unexpected audience: %#v", audience)
	}
	expStr, ok := summary["expiresAt"].(string)
	if !ok || expStr == "" {
		t.Fatalf("expected formatted expiry, got %#v", summary["expiresAt"])
	}
}

func TestWatcherAuthRejectLogsAndWrites(t *testing.T) {
	rr := httptest.NewRecorder()
	claims := jwt.MapClaims{"watcher_id": "w1"}
	watcherAuthReject(rr, http.StatusUnauthorized, "invalid", claims, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}
	body := rr.Body.String()
	if body == "" {
		t.Fatalf("expected body to contain message")
	}
}
