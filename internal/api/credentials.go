package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"kubeop/internal/service"
)

type credentialScopeRequest struct {
	Type string `json:"type"`
	ID   string `json:"id"`
}

type createGitCredentialRequest struct {
	Name  string                 `json:"name"`
	Scope credentialScopeRequest `json:"scope"`
	Auth  gitCredentialAuth      `json:"auth"`
}

type gitCredentialAuth struct {
	Type       string `json:"type"`
	Username   string `json:"username,omitempty"`
	Token      string `json:"token,omitempty"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"privateKey,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
}

type createRegistryCredentialRequest struct {
	Name     string                 `json:"name"`
	Registry string                 `json:"registry"`
	Scope    credentialScopeRequest `json:"scope"`
	Auth     registryCredentialAuth `json:"auth"`
}

type registryCredentialAuth struct {
	Type     string `json:"type"`
	Username string `json:"username,omitempty"`
	Token    string `json:"token,omitempty"`
	Password string `json:"password,omitempty"`
}

func (a *API) createGitCredential(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "createGitCredential")
	if !ok {
		return
	}
	var req createGitCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	out, err := svc.CreateGitCredential(r.Context(), service.GitCredentialCreateInput{
		Name: strings.TrimSpace(req.Name),
		Scope: service.CredentialScopeInput{
			Type: req.Scope.Type,
			ID:   req.Scope.ID,
		},
		AuthType:   req.Auth.Type,
		Username:   req.Auth.Username,
		Token:      req.Auth.Token,
		Password:   req.Auth.Password,
		PrivateKey: req.Auth.PrivateKey,
		Passphrase: req.Auth.Passphrase,
	})
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (a *API) listGitCredentials(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "listGitCredentials")
	if !ok {
		return
	}
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	projectID := strings.TrimSpace(r.URL.Query().Get("projectId"))
	creds, err := svc.ListGitCredentials(r.Context(), service.CredentialListInput{UserID: userID, ProjectID: projectID})
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, creds)
}

func (a *API) getGitCredential(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "getGitCredential")
	if !ok {
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	out, err := svc.GetGitCredential(r.Context(), id)
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

func (a *API) deleteGitCredential(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "deleteGitCredential")
	if !ok {
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if err := svc.DeleteGitCredential(r.Context(), id); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) createRegistryCredential(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "createRegistryCredential")
	if !ok {
		return
	}
	var req createRegistryCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	out, err := svc.CreateRegistryCredential(r.Context(), service.RegistryCredentialCreateInput{
		Name:     strings.TrimSpace(req.Name),
		Registry: strings.TrimSpace(req.Registry),
		Scope: service.CredentialScopeInput{
			Type: req.Scope.Type,
			ID:   req.Scope.ID,
		},
		AuthType: req.Auth.Type,
		Username: req.Auth.Username,
		Token:    req.Auth.Token,
		Password: req.Auth.Password,
	})
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, out)
}

func (a *API) listRegistryCredentials(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "listRegistryCredentials")
	if !ok {
		return
	}
	userID := strings.TrimSpace(r.URL.Query().Get("userId"))
	projectID := strings.TrimSpace(r.URL.Query().Get("projectId"))
	creds, err := svc.ListRegistryCredentials(r.Context(), service.CredentialListInput{UserID: userID, ProjectID: projectID})
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, creds)
}

func (a *API) getRegistryCredential(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "getRegistryCredential")
	if !ok {
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	out, err := svc.GetRegistryCredential(r.Context(), id)
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

func (a *API) deleteRegistryCredential(w http.ResponseWriter, r *http.Request) {
	svc, ok := a.serviceOrError(w, "deleteRegistryCredential")
	if !ok {
		return
	}
	id := strings.TrimSpace(chi.URLParam(r, "id"))
	if err := svc.DeleteRegistryCredential(r.Context(), id); err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, sql.ErrNoRows) {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
