package testcase

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"kubeop/internal/api"
	"kubeop/internal/config"
)

func TestCredentialRoutes_Exist(t *testing.T) {
	cfg := &config.Config{DisableAuth: true}
	r := api.NewRouter(cfg, nil)

	// Git credentials
	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/v1/credentials/git", `{"name":"example","scope":{"type":"user","id":"u"},"auth":{"type":"token","token":"abc"}}`},
		{http.MethodGet, "/v1/credentials/git", ""},
		{http.MethodGet, "/v1/credentials/git/id-1", ""},
		{http.MethodDelete, "/v1/credentials/git/id-1", ""},
		{http.MethodPost, "/v1/credentials/registries", `{"name":"dockerhub","registry":"https://index.docker.io/v1/","scope":{"type":"project","id":"p"},"auth":{"type":"basic","username":"u","password":"p"}}`},
		{http.MethodGet, "/v1/credentials/registries", ""},
		{http.MethodGet, "/v1/credentials/registries/id-2", ""},
		{http.MethodDelete, "/v1/credentials/registries/id-2", ""},
	}

	for _, tc := range cases {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusInternalServerError { // service nil
			t.Fatalf("%s %s expected 500, got %d", tc.method, tc.path, rr.Code)
		}
	}
}
