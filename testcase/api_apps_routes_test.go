package testcase

import (
    "bytes"
    "net/http"
    "net/http/httptest"
    "testing"
    "kubeop/internal/api"
    "kubeop/internal/config"
)

func routerNoSvc() http.Handler { return api.NewRouter(&config.Config{DisableAuth: true}, nil) }

func TestAppsRoutes_ListAndGet(t *testing.T) {
    r := routerNoSvc()
    // list apps
    rr := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodGet, "/v1/projects/p1/apps", nil)
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusInternalServerError { t.Fatalf("GET /v1/projects/{id}/apps expected 500, got %d", rr.Code) }

    // get app status
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodGet, "/v1/projects/p1/apps/a1", nil)
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusInternalServerError { t.Fatalf("GET /v1/projects/{id}/apps/{appId} expected 500, got %d", rr.Code) }
}

func TestAppsRoutes_ScaleImageRollout(t *testing.T) {
    r := routerNoSvc()
    // scale
    rr := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodPatch, "/v1/projects/p1/apps/a1/scale", bytes.NewBufferString(`{"replicas":1}`))
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusInternalServerError { t.Fatalf("PATCH scale expected 500, got %d", rr.Code) }
    // image
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodPatch, "/v1/projects/p1/apps/a1/image", bytes.NewBufferString(`{"image":"nginx"}`))
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusInternalServerError { t.Fatalf("PATCH image expected 500, got %d", rr.Code) }
    // rollout restart
    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodPost, "/v1/projects/p1/apps/a1/rollout/restart", nil)
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusInternalServerError { t.Fatalf("POST rollout restart expected 500, got %d", rr.Code) }
}

func TestConfigsSecretsRoutes(t *testing.T) {
    r := routerNoSvc()
    rr := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodPost, "/v1/projects/p1/configs", bytes.NewBufferString(`{"name":"c","data":{}}`))
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusInternalServerError { t.Fatalf("POST configs expected 500, got %d", rr.Code) }

    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodGet, "/v1/projects/p1/configs", nil)
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusInternalServerError { t.Fatalf("GET configs expected 500, got %d", rr.Code) }

    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodDelete, "/v1/projects/p1/configs/name", nil)
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusInternalServerError { t.Fatalf("DELETE configs expected 500, got %d", rr.Code) }

    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodPost, "/v1/projects/p1/secrets", bytes.NewBufferString(`{"name":"s","stringData":{}}`))
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusInternalServerError { t.Fatalf("POST secrets expected 500, got %d", rr.Code) }

    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodGet, "/v1/projects/p1/secrets", nil)
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusInternalServerError { t.Fatalf("GET secrets expected 500, got %d", rr.Code) }

    rr = httptest.NewRecorder()
    req = httptest.NewRequest(http.MethodDelete, "/v1/projects/p1/secrets/name", nil)
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusInternalServerError { t.Fatalf("DELETE secrets expected 500, got %d", rr.Code) }
}

func TestUserRenewRoute(t *testing.T) {
    r := routerNoSvc()
    rr := httptest.NewRecorder()
    req := httptest.NewRequest(http.MethodPost, "/v1/users/u1/kubeconfig/renew", bytes.NewBufferString(`{"clusterId":"c1"}`))
    r.ServeHTTP(rr, req)
    if rr.Code != http.StatusInternalServerError { t.Fatalf("POST /v1/users/{id}/kubeconfig/renew expected 500, got %d", rr.Code) }
}

