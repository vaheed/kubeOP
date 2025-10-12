package testcase

import (
	"errors"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"

	"kubeop/internal/store"
)

func TestFormatMigrateErrorDirtyDetails(t *testing.T) {
	dirty := migrate.ErrDirty{Version: 8}
	err := store.FormatMigrateError(dirty)
	if err == nil {
		t.Fatalf("expected formatted error for dirty migration")
	}
	msg := err.Error()
	if !strings.Contains(msg, "dirty database at version 8") {
		t.Fatalf("expected dirty version note, got %q", msg)
	}
	if !strings.Contains(msg, "migrate force 8") {
		t.Fatalf("expected migrate force hint, got %q", msg)
	}
}

func TestFormatMigrateErrorGeneral(t *testing.T) {
	base := errors.New("boom")
	err := store.FormatMigrateError(base)
	if err == nil {
		t.Fatalf("expected formatted error")
	}
	if !strings.Contains(err.Error(), "migrate up") {
		t.Fatalf("expected migrate up prefix, got %q", err.Error())
	}
}

func TestFormatMigrateErrorNil(t *testing.T) {
	if err := store.FormatMigrateError(nil); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}
