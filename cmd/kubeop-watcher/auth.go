package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"kubeop/internal/sink"
	"kubeop/internal/state"
	"kubeop/internal/watcher/authutil"
)

const (
	accessRefreshSkew    = 30 * time.Second
	refreshExpirySkew    = 5 * time.Minute
	unauthorizedCooldown = 5 * time.Second
)

type authManager struct {
	cfg         watcherConfig
	store       *state.Store
	sink        *sink.Sink
	client      *http.Client
	logger      *zap.Logger
	mu          sync.RWMutex
	creds       state.Credentials
	haveCreds   bool
	readyOnce   sync.Once
	readyCh     chan struct{}
	signalCh    chan struct{}
	lastRefresh time.Time
	nextAccess  time.Time
}

func newAuthManager(cfg watcherConfig, store *state.Store, sink *sink.Sink, logger *zap.Logger) *authManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &authManager{
		cfg:      cfg,
		store:    store,
		sink:     sink,
		client:   &http.Client{Timeout: 15 * time.Second},
		logger:   logger,
		readyCh:  make(chan struct{}),
		signalCh: make(chan struct{}, 1),
	}
}

func (m *authManager) AttachSink(s *sink.Sink) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sink = s
	if m.haveCreds && m.sink != nil {
		m.sink.SetToken(m.creds.AccessToken)
	}
}

func (m *authManager) Initialize(ctx context.Context) error {
	if m == nil {
		return errors.New("auth manager nil")
	}
	if err := m.ensureCredentials(ctx); err != nil {
		return err
	}
	m.startReady()
	go m.run(ctx)
	return nil
}

func (m *authManager) startReady() {
	m.readyOnce.Do(func() {
		close(m.readyCh)
	})
}

func (m *authManager) WaitReady(ctx context.Context) error {
	if m == nil {
		return errors.New("auth manager nil")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-m.readyCh:
		return nil
	}
}

func (m *authManager) ForceRefresh(ctx context.Context) error {
	if m == nil {
		return errors.New("auth manager nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.refreshLockedInternal(ctx, true)
}

func (m *authManager) run(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := m.ensureCredentials(ctx); err != nil {
				m.logger.Warn("auth refresh failed", zap.Error(err))
			}
		case <-m.signalCh:
			if err := m.refresh(ctx, true); err != nil {
				m.logger.Warn("forced auth refresh failed", zap.Error(err))
			}
			time.Sleep(unauthorizedCooldown)
		}
	}
}

func (m *authManager) ensureCredentials(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.haveCreds {
		if err := m.loadFromStoreLocked(); err != nil {
			return err
		}
	}
	now := time.Now()
	needsRegister := !m.haveCreds || m.creds.RefreshToken == "" || m.creds.WatcherID == ""
	if !needsRegister && !m.creds.RefreshExpires.IsZero() && now.After(m.creds.RefreshExpires.Add(-refreshExpirySkew)) {
		needsRegister = true
	}
	if needsRegister {
		if err := m.registerLocked(ctx); err != nil {
			return err
		}
		return nil
	}
	if !m.creds.AccessExpires.IsZero() {
		if !m.nextAccess.IsZero() && now.After(m.nextAccess) {
			return m.refreshLocked(ctx)
		}
		if now.After(m.creds.AccessExpires.Add(-accessRefreshSkew)) {
			return m.refreshLocked(ctx)
		}
	}
	return nil
}

func (m *authManager) registerLocked(ctx context.Context) error {
	if m == nil {
		return errors.New("auth manager nil")
	}
	payload := map[string]string{"cluster_id": m.cfg.ClusterID}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal register payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.cfg.RegisterURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build register request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(m.cfg.BootstrapToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("register watcher: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return fmt.Errorf("register watcher unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	creds, err := decodeCredentialResponse(resp)
	if err != nil {
		return err
	}
	if err := m.persistLocked(creds); err != nil {
		return err
	}
	m.logger.Info("registered watcher", zap.String("watcher_id", creds.WatcherID))
	return nil
}

func (m *authManager) refreshLocked(ctx context.Context) error {
	return m.refreshLockedInternal(ctx, false)
}

func (m *authManager) refresh(ctx context.Context, force bool) error {
	if m == nil {
		return errors.New("auth manager nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.refreshLockedInternal(ctx, force)
}

func (m *authManager) refreshLockedInternal(ctx context.Context, force bool) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if !m.haveCreds {
		if err := m.loadFromStoreLocked(); err != nil {
			return err
		}
	}
	if !m.haveCreds {
		return m.registerLocked(ctx)
	}
	watcherID := strings.TrimSpace(m.creds.WatcherID)
	refreshToken := strings.TrimSpace(m.creds.RefreshToken)
	if watcherID == "" || refreshToken == "" {
		return m.registerLocked(ctx)
	}
	if !m.creds.RefreshExpires.IsZero() && time.Now().After(m.creds.RefreshExpires) {
		return m.registerLocked(ctx)
	}
	payload := map[string]string{
		"watcher_id":    watcherID,
		"refresh_token": refreshToken,
		"cluster_id":    m.cfg.ClusterID,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal refresh payload: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.cfg.RefreshURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("refresh watcher token: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			m.logger.Warn("refresh unauthorized", zap.Int("status", resp.StatusCode))
			return m.registerLocked(ctx)
		}
		return fmt.Errorf("refresh watcher unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}
	creds, err := decodeCredentialResponse(resp)
	if err != nil {
		return err
	}
	if err := m.persistLocked(creds); err != nil {
		return err
	}
	m.logger.Info("rotated watcher credentials", zap.String("watcher_id", creds.WatcherID))
	return nil
}

func (m *authManager) loadFromStoreLocked() error {
	if m.store == nil {
		return errors.New("state store not initialised")
	}
	creds, ok, err := m.store.LoadCredentials()
	if err != nil {
		return err
	}
	if !ok {
		m.haveCreds = false
		return nil
	}
	m.creds = creds
	m.haveCreds = true
	m.nextAccess = authutil.NextAccessRefresh(time.Now(), creds.AccessExpires)
	if m.sink != nil {
		m.sink.SetToken(creds.AccessToken)
	}
	return nil
}

func (m *authManager) persistLocked(creds state.Credentials) error {
	if m.store == nil {
		return errors.New("state store not initialised")
	}
	if err := m.store.SaveCredentials(creds); err != nil {
		return err
	}
	m.creds = creds
	m.haveCreds = true
	m.nextAccess = authutil.NextAccessRefresh(time.Now(), creds.AccessExpires)
	if m.sink != nil {
		m.sink.SetToken(creds.AccessToken)
	}
	m.startReady()
	return nil
}

func (m *authManager) AccessToken() string {
	if m == nil {
		return ""
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.creds.AccessToken
}

func (m *authManager) SignalUnauthorized() {
	if m == nil {
		return
	}
	select {
	case m.signalCh <- struct{}{}:
	default:
	}
}

func decodeCredentialResponse(resp *http.Response) (state.Credentials, error) {
	if resp == nil {
		return state.Credentials{}, errors.New("nil response")
	}
	var payload struct {
		WatcherID      string `json:"watcherId"`
		ClusterID      string `json:"clusterId"`
		AccessToken    string `json:"accessToken"`
		AccessExpires  string `json:"accessExpiresAt"`
		RefreshToken   string `json:"refreshToken"`
		RefreshExpires string `json:"refreshExpiresAt"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return state.Credentials{}, fmt.Errorf("decode credentials: %w", err)
	}
	accessExp, err := time.Parse(time.RFC3339, payload.AccessExpires)
	if err != nil {
		return state.Credentials{}, fmt.Errorf("parse access expiry: %w", err)
	}
	refreshExp, err := time.Parse(time.RFC3339, payload.RefreshExpires)
	if err != nil {
		return state.Credentials{}, fmt.Errorf("parse refresh expiry: %w", err)
	}
	return state.Credentials{
		WatcherID:      payload.WatcherID,
		AccessToken:    payload.AccessToken,
		AccessExpires:  accessExp,
		RefreshToken:   payload.RefreshToken,
		RefreshExpires: refreshExp,
	}, nil
}
