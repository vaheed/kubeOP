package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"kubeop/internal/store"
)

// ErrMaintenanceEnabled indicates that maintenance mode is currently enabled.
var ErrMaintenanceEnabled = errors.New("maintenance mode enabled")

// ErrInvalidMaintenanceInput signals invalid update payloads.
var ErrInvalidMaintenanceInput = errors.New("invalid maintenance input")

// MaintenanceError wraps ErrMaintenanceEnabled with an optional operator-provided message.
type MaintenanceError struct {
	Message string
}

// Error implements error.
func (e MaintenanceError) Error() string {
	base := ErrMaintenanceEnabled.Error()
	if strings.TrimSpace(e.Message) == "" {
		return base
	}
	return fmt.Sprintf("%s: %s", base, strings.TrimSpace(e.Message))
}

// Is enables errors.Is compatibility with ErrMaintenanceEnabled.
func (e MaintenanceError) Is(target error) bool {
	return target == ErrMaintenanceEnabled
}

// MaintenanceState represents the user-facing maintenance toggle payload.
type MaintenanceState struct {
	Enabled   bool      `json:"enabled"`
	Message   string    `json:"message"`
	UpdatedAt time.Time `json:"updatedAt"`
	UpdatedBy string    `json:"updatedBy"`
}

// MaintenanceUpdateInput captures toggle updates from operators.
type MaintenanceUpdateInput struct {
	Enabled bool
	Message string
}

// GetMaintenanceState fetches the persisted maintenance toggle.
func (s *Service) GetMaintenanceState(ctx context.Context) (MaintenanceState, error) {
	state, err := s.maintenanceState(ctx)
	if err != nil {
		return MaintenanceState{}, err
	}
	return MaintenanceState{
		Enabled:   state.Enabled,
		Message:   state.Message,
		UpdatedAt: state.UpdatedAt,
		UpdatedBy: state.UpdatedBy,
	}, nil
}

// UpdateMaintenanceState toggles maintenance mode and records actor metadata.
func (s *Service) UpdateMaintenanceState(ctx context.Context, in MaintenanceUpdateInput) (MaintenanceState, error) {
	msg := strings.TrimSpace(in.Message)
	if len(msg) > 512 {
		return MaintenanceState{}, fmt.Errorf("%w: message must be 512 characters or fewer", ErrInvalidMaintenanceInput)
	}
	actor := strings.TrimSpace(actorFromContext(ctx))
	if actor == "" {
		actor = "system"
	}
	state, err := s.st.UpdateMaintenanceState(ctx, in.Enabled, msg, actor)
	if err != nil {
		return MaintenanceState{}, err
	}
	logger := s.logger
	if logger == nil {
		logger = zap.NewNop()
	}
	logger.Info(
		"maintenance_mode_updated",
		zap.Bool("enabled", state.Enabled),
		zap.String("message", state.Message),
		zap.String("actor", state.UpdatedBy),
	)
	return MaintenanceState{
		Enabled:   state.Enabled,
		Message:   state.Message,
		UpdatedAt: state.UpdatedAt,
		UpdatedBy: state.UpdatedBy,
	}, nil
}

func (s *Service) maintenanceState(ctx context.Context) (store.MaintenanceState, error) {
	loader := s.maintenanceLoader
	if loader == nil {
		loader = s.st.GetMaintenanceState
	}
	return loader(ctx)
}

func (s *Service) ensureMaintenanceAllows(ctx context.Context, action string) error {
	state, err := s.maintenanceState(ctx)
	if err != nil {
		return err
	}
	if !state.Enabled {
		return nil
	}
	logger := s.logger
	if logger == nil {
		logger = zap.NewNop()
	}
	logger.Info(
		"maintenance_mode_block",
		zap.String("action", action),
		zap.String("actor", actorFromContext(ctx)),
		zap.String("message", strings.TrimSpace(state.Message)),
	)
	return MaintenanceError{Message: state.Message}
}
