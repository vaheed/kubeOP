package testcase

import (
	"context"
	"testing"

	"kubeop/internal/service"
	"kubeop/internal/store"
)

func disableMaintenance(t *testing.T, svc *service.Service) {
	t.Helper()
	svc.SetMaintenanceLoader(func(ctx context.Context) (store.MaintenanceState, error) {
		return store.MaintenanceState{}, nil
	})
}
