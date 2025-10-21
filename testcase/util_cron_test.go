package testcase

import (
	"errors"
	"testing"

	batchv1 "k8s.io/api/batch/v1"

	"kubeop/internal/util"
)

func TestValidateCronConfig_Defaults(t *testing.T) {
	res, err := util.ValidateCronConfig(util.CronConfig{Schedule: "*/5 * * * *"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Schedule != "*/5 * * * *" {
		t.Fatalf("expected schedule to be preserved, got %q", res.Schedule)
	}
	if res.ConcurrencyPolicy != batchv1.AllowConcurrent {
		t.Fatalf("expected default concurrency policy Allow, got %s", res.ConcurrencyPolicy)
	}
	if res.HasExplicitConcurrency {
		t.Fatalf("expected HasExplicitConcurrency to be false")
	}
	if res.HasExplicitTimeZone {
		t.Fatalf("expected HasExplicitTimeZone to be false")
	}
	if res.Location != nil {
		t.Fatalf("expected no location when timezone omitted")
	}
}

func TestValidateCronConfig_WithTimezoneAndPolicy(t *testing.T) {
	res, err := util.ValidateCronConfig(util.CronConfig{
		Schedule:          "0 3 * * *",
		TimeZone:          "America/New_York",
		ConcurrencyPolicy: string(batchv1.ForbidConcurrent),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.HasExplicitTimeZone {
		t.Fatalf("expected timezone to be flagged as explicit")
	}
	if res.Location == nil || res.Location.String() != "America/New_York" {
		t.Fatalf("expected timezone location to resolve to America/New_York, got %#v", res.Location)
	}
	if !res.HasExplicitConcurrency {
		t.Fatalf("expected concurrency policy to be flagged as explicit")
	}
	if res.ConcurrencyPolicy != batchv1.ForbidConcurrent {
		t.Fatalf("expected concurrency policy Forbid, got %s", res.ConcurrencyPolicy)
	}
}

func TestValidateCronConfig_InvalidSchedule(t *testing.T) {
	_, err := util.ValidateCronConfig(util.CronConfig{Schedule: "not-a-schedule"})
	if !errors.Is(err, util.ErrCronScheduleInvalid) {
		t.Fatalf("expected ErrCronScheduleInvalid, got %v", err)
	}
}

func TestValidateCronConfig_ScheduleRequired(t *testing.T) {
	_, err := util.ValidateCronConfig(util.CronConfig{})
	if !errors.Is(err, util.ErrCronScheduleRequired) {
		t.Fatalf("expected ErrCronScheduleRequired, got %v", err)
	}
}

func TestValidateCronConfig_DescriptorUnsupported(t *testing.T) {
	_, err := util.ValidateCronConfig(util.CronConfig{Schedule: "@every 5m"})
	if !errors.Is(err, util.ErrCronDescriptorUnsupported) {
		t.Fatalf("expected ErrCronDescriptorUnsupported, got %v", err)
	}
}

func TestValidateCronConfig_InvalidTimeZone(t *testing.T) {
	_, err := util.ValidateCronConfig(util.CronConfig{Schedule: "0 12 * * *", TimeZone: "Mars/Olympus"})
	if !errors.Is(err, util.ErrCronTimeZoneInvalid) {
		t.Fatalf("expected ErrCronTimeZoneInvalid, got %v", err)
	}
}

func TestValidateCronConfig_InvalidConcurrencyPolicy(t *testing.T) {
	_, err := util.ValidateCronConfig(util.CronConfig{Schedule: "0 12 * * *", ConcurrencyPolicy: "Parallel"})
	if !errors.Is(err, util.ErrCronConcurrencyPolicyInvalid) {
		t.Fatalf("expected ErrCronConcurrencyPolicyInvalid, got %v", err)
	}
}
