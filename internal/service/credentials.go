package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"kubeop/internal/crypto"
	"kubeop/internal/store"
)

type CredentialScope string

const (
	CredentialScopeUser    CredentialScope = "USER"
	CredentialScopeProject CredentialScope = "PROJECT"
)

type CredentialScopeInput struct {
	Type string
	ID   string
}

type CredentialListInput struct {
	UserID    string
	ProjectID string
}

type GitCredentialCreateInput struct {
	Name       string
	Scope      CredentialScopeInput
	AuthType   string
	Username   string
	Token      string
	Password   string
	PrivateKey string
	Passphrase string
}

type GitCredentialOutput struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	ScopeType CredentialScope `json:"scopeType"`
	ScopeID   string          `json:"scopeId"`
	AuthType  string          `json:"authType"`
	Username  string          `json:"username,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

type GitCredentialSecret struct {
	Token      string `json:"token,omitempty"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"privateKey,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
}

type GitCredentialSecretOutput struct {
	GitCredentialOutput
	Secret GitCredentialSecret `json:"secret"`
}

type RegistryCredentialCreateInput struct {
	Name     string
	Registry string
	Scope    CredentialScopeInput
	AuthType string
	Username string
	Token    string
	Password string
}

type RegistryCredentialOutput struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Registry  string          `json:"registry"`
	ScopeType CredentialScope `json:"scopeType"`
	ScopeID   string          `json:"scopeId"`
	AuthType  string          `json:"authType"`
	Username  string          `json:"username,omitempty"`
	CreatedAt time.Time       `json:"createdAt"`
	UpdatedAt time.Time       `json:"updatedAt"`
}

type RegistryCredentialSecret struct {
	Token    string `json:"token,omitempty"`
	Password string `json:"password,omitempty"`
}

type RegistryCredentialSecretOutput struct {
	RegistryCredentialOutput
	Secret RegistryCredentialSecret `json:"secret"`
}

type gitCredentialSecretPayload struct {
	Token      string `json:"token,omitempty"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"privateKey,omitempty"`
	Passphrase string `json:"passphrase,omitempty"`
}

type registryCredentialSecretPayload struct {
	Token    string `json:"token,omitempty"`
	Password string `json:"password,omitempty"`
}

func (s *Service) CreateGitCredential(ctx context.Context, in GitCredentialCreateInput) (GitCredentialOutput, error) {
	if s == nil {
		return GitCredentialOutput{}, errors.New("service unavailable")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return GitCredentialOutput{}, errors.New("name is required")
	}
	authType := strings.ToUpper(strings.TrimSpace(in.AuthType))
	if authType == "" {
		return GitCredentialOutput{}, errors.New("authType is required")
	}
	var payload gitCredentialSecretPayload
	username := strings.TrimSpace(in.Username)
	switch authType {
	case "TOKEN":
		tok := strings.TrimSpace(in.Token)
		if tok == "" {
			return GitCredentialOutput{}, errors.New("token is required for TOKEN authType")
		}
		payload.Token = tok
	case "BASIC":
		if username == "" {
			return GitCredentialOutput{}, errors.New("username is required for BASIC authType")
		}
		pass := strings.TrimSpace(in.Password)
		if pass == "" {
			return GitCredentialOutput{}, errors.New("password is required for BASIC authType")
		}
		payload.Password = pass
	case "SSH":
		key := strings.TrimSpace(in.PrivateKey)
		if key == "" {
			return GitCredentialOutput{}, errors.New("privateKey is required for SSH authType")
		}
		payload.PrivateKey = key
		if pass := strings.TrimSpace(in.Passphrase); pass != "" {
			payload.Passphrase = pass
		}
	default:
		return GitCredentialOutput{}, fmt.Errorf("unsupported authType %q", in.AuthType)
	}
	userIDPtr, projectIDPtr, scopeType, scopeID, err := s.resolveCredentialScope(ctx, in.Scope)
	if err != nil {
		return GitCredentialOutput{}, err
	}
	enc, err := s.encryptCredentialPayload(payload)
	if err != nil {
		return GitCredentialOutput{}, err
	}
	rec := store.GitCredential{
		ID:        uuid.New().String(),
		Name:      name,
		UserID:    userIDPtr,
		ProjectID: projectIDPtr,
		AuthType:  authType,
		Username:  username,
	}
	created, err := s.st.CreateGitCredential(ctx, rec, enc)
	if err != nil {
		return GitCredentialOutput{}, err
	}
	s.logger.Info(
		"git_credential_created",
		zap.String("credential_id", created.ID),
		zap.String("auth_type", created.AuthType),
		zap.String("scope_type", string(scopeType)),
		zap.String("scope_id", scopeID),
	)
	return GitCredentialOutput{
		ID:        created.ID,
		Name:      created.Name,
		ScopeType: scopeType,
		ScopeID:   scopeID,
		AuthType:  created.AuthType,
		Username:  created.Username,
		CreatedAt: created.CreatedAt,
		UpdatedAt: created.UpdatedAt,
	}, nil
}

func (s *Service) ListGitCredentials(ctx context.Context, in CredentialListInput) ([]GitCredentialOutput, error) {
	if s == nil {
		return nil, errors.New("service unavailable")
	}
	if in.UserID != "" && in.ProjectID != "" {
		return nil, errors.New("provide either userId or projectId, not both")
	}
	filter := store.CredentialFilter{UserID: strings.TrimSpace(in.UserID), ProjectID: strings.TrimSpace(in.ProjectID)}
	creds, err := s.st.ListGitCredentials(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]GitCredentialOutput, 0, len(creds))
	for _, c := range creds {
		scopeType, scopeID := mapCredentialScope(c.UserID, c.ProjectID)
		out = append(out, GitCredentialOutput{
			ID:        c.ID,
			Name:      c.Name,
			ScopeType: scopeType,
			ScopeID:   scopeID,
			AuthType:  c.AuthType,
			Username:  c.Username,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
		})
	}
	return out, nil
}

func (s *Service) GetGitCredential(ctx context.Context, id string) (GitCredentialSecretOutput, error) {
	if s == nil {
		return GitCredentialSecretOutput{}, errors.New("service unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return GitCredentialSecretOutput{}, errors.New("id is required")
	}
	rec, secretEnc, err := s.st.GetGitCredential(ctx, id)
	if err != nil {
		return GitCredentialSecretOutput{}, err
	}
	raw, err := crypto.DecryptAESGCM(secretEnc, s.encKey)
	if err != nil {
		return GitCredentialSecretOutput{}, err
	}
	var secret GitCredentialSecret
	if err := json.Unmarshal(raw, &secret); err != nil {
		return GitCredentialSecretOutput{}, err
	}
	scopeType, scopeID := mapCredentialScope(rec.UserID, rec.ProjectID)
	return GitCredentialSecretOutput{
		GitCredentialOutput: GitCredentialOutput{
			ID:        rec.ID,
			Name:      rec.Name,
			ScopeType: scopeType,
			ScopeID:   scopeID,
			AuthType:  rec.AuthType,
			Username:  rec.Username,
			CreatedAt: rec.CreatedAt,
			UpdatedAt: rec.UpdatedAt,
		},
		Secret: secret,
	}, nil
}

func (s *Service) DeleteGitCredential(ctx context.Context, id string) error {
	if s == nil {
		return errors.New("service unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("id is required")
	}
	rec, secret, err := s.st.GetGitCredential(ctx, id)
	if err != nil {
		return err
	}
	_ = secret
	if err := s.st.DeleteGitCredential(ctx, id); err != nil {
		return err
	}
	scopeType, scopeID := mapCredentialScope(rec.UserID, rec.ProjectID)
	s.logger.Info(
		"git_credential_deleted",
		zap.String("credential_id", rec.ID),
		zap.String("scope_type", string(scopeType)),
		zap.String("scope_id", scopeID),
	)
	return nil
}

func (s *Service) CreateRegistryCredential(ctx context.Context, in RegistryCredentialCreateInput) (RegistryCredentialOutput, error) {
	if s == nil {
		return RegistryCredentialOutput{}, errors.New("service unavailable")
	}
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return RegistryCredentialOutput{}, errors.New("name is required")
	}
	registry := strings.TrimSpace(in.Registry)
	if registry == "" {
		return RegistryCredentialOutput{}, errors.New("registry is required")
	}
	authType := strings.ToUpper(strings.TrimSpace(in.AuthType))
	if authType == "" {
		return RegistryCredentialOutput{}, errors.New("authType is required")
	}
	username := strings.TrimSpace(in.Username)
	var payload registryCredentialSecretPayload
	switch authType {
	case "TOKEN":
		tok := strings.TrimSpace(in.Token)
		if tok == "" {
			return RegistryCredentialOutput{}, errors.New("token is required for TOKEN authType")
		}
		payload.Token = tok
	case "BASIC":
		if username == "" {
			return RegistryCredentialOutput{}, errors.New("username is required for BASIC authType")
		}
		pass := strings.TrimSpace(in.Password)
		if pass == "" {
			return RegistryCredentialOutput{}, errors.New("password is required for BASIC authType")
		}
		payload.Password = pass
	default:
		return RegistryCredentialOutput{}, fmt.Errorf("unsupported authType %q", in.AuthType)
	}
	userIDPtr, projectIDPtr, scopeType, scopeID, err := s.resolveCredentialScope(ctx, in.Scope)
	if err != nil {
		return RegistryCredentialOutput{}, err
	}
	enc, err := s.encryptCredentialPayload(payload)
	if err != nil {
		return RegistryCredentialOutput{}, err
	}
	rec := store.RegistryCredential{
		ID:        uuid.New().String(),
		Name:      name,
		Registry:  registry,
		UserID:    userIDPtr,
		ProjectID: projectIDPtr,
		AuthType:  authType,
		Username:  username,
	}
	created, err := s.st.CreateRegistryCredential(ctx, rec, enc)
	if err != nil {
		return RegistryCredentialOutput{}, err
	}
	s.logger.Info(
		"registry_credential_created",
		zap.String("credential_id", created.ID),
		zap.String("registry", created.Registry),
		zap.String("auth_type", created.AuthType),
		zap.String("scope_type", string(scopeType)),
		zap.String("scope_id", scopeID),
	)
	return RegistryCredentialOutput{
		ID:        created.ID,
		Name:      created.Name,
		Registry:  created.Registry,
		ScopeType: scopeType,
		ScopeID:   scopeID,
		AuthType:  created.AuthType,
		Username:  created.Username,
		CreatedAt: created.CreatedAt,
		UpdatedAt: created.UpdatedAt,
	}, nil
}

func (s *Service) ListRegistryCredentials(ctx context.Context, in CredentialListInput) ([]RegistryCredentialOutput, error) {
	if s == nil {
		return nil, errors.New("service unavailable")
	}
	if in.UserID != "" && in.ProjectID != "" {
		return nil, errors.New("provide either userId or projectId, not both")
	}
	filter := store.CredentialFilter{UserID: strings.TrimSpace(in.UserID), ProjectID: strings.TrimSpace(in.ProjectID)}
	creds, err := s.st.ListRegistryCredentials(ctx, filter)
	if err != nil {
		return nil, err
	}
	out := make([]RegistryCredentialOutput, 0, len(creds))
	for _, c := range creds {
		scopeType, scopeID := mapCredentialScope(c.UserID, c.ProjectID)
		out = append(out, RegistryCredentialOutput{
			ID:        c.ID,
			Name:      c.Name,
			Registry:  c.Registry,
			ScopeType: scopeType,
			ScopeID:   scopeID,
			AuthType:  c.AuthType,
			Username:  c.Username,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
		})
	}
	return out, nil
}

func (s *Service) GetRegistryCredential(ctx context.Context, id string) (RegistryCredentialSecretOutput, error) {
	if s == nil {
		return RegistryCredentialSecretOutput{}, errors.New("service unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return RegistryCredentialSecretOutput{}, errors.New("id is required")
	}
	rec, secretEnc, err := s.st.GetRegistryCredential(ctx, id)
	if err != nil {
		return RegistryCredentialSecretOutput{}, err
	}
	raw, err := crypto.DecryptAESGCM(secretEnc, s.encKey)
	if err != nil {
		return RegistryCredentialSecretOutput{}, err
	}
	var secret RegistryCredentialSecret
	if err := json.Unmarshal(raw, &secret); err != nil {
		return RegistryCredentialSecretOutput{}, err
	}
	scopeType, scopeID := mapCredentialScope(rec.UserID, rec.ProjectID)
	return RegistryCredentialSecretOutput{
		RegistryCredentialOutput: RegistryCredentialOutput{
			ID:        rec.ID,
			Name:      rec.Name,
			Registry:  rec.Registry,
			ScopeType: scopeType,
			ScopeID:   scopeID,
			AuthType:  rec.AuthType,
			Username:  rec.Username,
			CreatedAt: rec.CreatedAt,
			UpdatedAt: rec.UpdatedAt,
		},
		Secret: secret,
	}, nil
}

func (s *Service) DeleteRegistryCredential(ctx context.Context, id string) error {
	if s == nil {
		return errors.New("service unavailable")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("id is required")
	}
	rec, secret, err := s.st.GetRegistryCredential(ctx, id)
	if err != nil {
		return err
	}
	_ = secret
	if err := s.st.DeleteRegistryCredential(ctx, id); err != nil {
		return err
	}
	scopeType, scopeID := mapCredentialScope(rec.UserID, rec.ProjectID)
	s.logger.Info(
		"registry_credential_deleted",
		zap.String("credential_id", rec.ID),
		zap.String("registry", rec.Registry),
		zap.String("scope_type", string(scopeType)),
		zap.String("scope_id", scopeID),
	)
	return nil
}

func (s *Service) resolveCredentialScope(ctx context.Context, scope CredentialScopeInput) (*string, *string, CredentialScope, string, error) {
	scopeType := CredentialScope(strings.ToUpper(strings.TrimSpace(scope.Type)))
	scopeID := strings.TrimSpace(scope.ID)
	if scopeID == "" {
		return nil, nil, "", "", errors.New("scope.id is required")
	}
	switch scopeType {
	case CredentialScopeUser:
		if _, err := s.st.GetUser(ctx, scopeID); err != nil {
			return nil, nil, "", "", err
		}
		userID := scopeID
		return &userID, nil, CredentialScopeUser, scopeID, nil
	case CredentialScopeProject:
		if _, _, _, err := s.st.GetProject(ctx, scopeID); err != nil {
			return nil, nil, "", "", err
		}
		projectID := scopeID
		return nil, &projectID, CredentialScopeProject, scopeID, nil
	default:
		return nil, nil, "", "", errors.New("scope.type must be user or project")
	}
}

func (s *Service) encryptCredentialPayload(payload any) ([]byte, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return crypto.EncryptAESGCM(raw, s.encKey)
}

func mapCredentialScope(userID, projectID *string) (CredentialScope, string) {
	if userID != nil {
		return CredentialScopeUser, *userID
	}
	if projectID != nil {
		return CredentialScopeProject, *projectID
	}
	return "", ""
}
