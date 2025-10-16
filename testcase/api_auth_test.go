package testcase

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/golang-jwt/jwt/v5"
	"kubeop/internal/api"
	"kubeop/internal/config"
)

func TestAdminAuthMiddleware(t *testing.T) {
	cfg := &config.Config{AdminJWTSecret: "topsecret", DisableAuth: false}
	mw := api.AdminAuthMiddleware(cfg)

	// target handler records status 200
	target := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	h := mw(target)

	// 1) Missing header -> 401
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/secure", nil)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	// 2) Invalid token -> 401
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/secure", nil)
	req.Header.Set("Authorization", "Bearer invalid")
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rr.Code)
	}

	// 3) Wrong role -> 403
	badTok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"role": "user"})
	badStr, _ := badTok.SignedString([]byte(cfg.AdminJWTSecret))
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/secure", nil)
	req.Header.Set("Authorization", "Bearer "+badStr)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rr.Code)
	}

	// 4) Valid token -> 200
	goodTok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"role": "admin"})
	goodStr, _ := goodTok.SignedString([]byte(cfg.AdminJWTSecret))
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/v1/secure", nil)
	req.Header.Set("Authorization", "Bearer "+goodStr)
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestWatcherHandshakeReturnsClusterID(t *testing.T) {
	cfg := &config.Config{AdminJWTSecret: "secret", DisableAuth: false}
	router := api.NewRouter(cfg, nil)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"role":       "admin",
		"cluster_id": "cluster-1",
		"sub":        "watcher:cluster-1",
	})
	tokenStr, err := token.SignedString([]byte(cfg.AdminJWTSecret))
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}

	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/watchers/handshake", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected handshake 200, got %d", rr.Code)
	}
	expected := "\"cluster-1\""
	if !strings.Contains(rr.Body.String(), expected) {
		t.Fatalf("expected response to contain cluster id %s, got %s", expected, rr.Body.String())
	}

	// Missing cluster ID claim should fail.
	emptyToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"role": "admin"})
	emptyStr, _ := emptyToken.SignedString([]byte(cfg.AdminJWTSecret))
	rr = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/v1/watchers/handshake", nil)
	req.Header.Set("Authorization", "Bearer "+emptyStr)
	router.ServeHTTP(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing cluster id, got %d", rr.Code)
	}
}
