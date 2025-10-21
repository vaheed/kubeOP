package util

import (
	"errors"
	"fmt"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"

	"github.com/robfig/cron/v3"
)

var (
	// ErrCronScheduleRequired indicates that a cron schedule string was not provided.
	ErrCronScheduleRequired = errors.New("cron schedule is required")
	// ErrCronScheduleInvalid wraps parsing errors from the cron library.
	ErrCronScheduleInvalid = errors.New("invalid cron schedule")
	// ErrCronDescriptorUnsupported reports when unsupported descriptors such as @every are used.
	ErrCronDescriptorUnsupported = errors.New("@every descriptor is not supported for Kubernetes CronJobs")
	// ErrCronTimeZoneInvalid wraps failures when resolving an IANA timezone identifier.
	ErrCronTimeZoneInvalid = errors.New("invalid cron timezone")
	// ErrCronConcurrencyPolicyInvalid reports when an unsupported concurrency policy was requested.
	ErrCronConcurrencyPolicyInvalid = errors.New("invalid cron concurrency policy")
)

var scheduleParser = cron.NewParser(
	cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

// CronConfig captures the user-provided cron job schedule settings that require validation.
type CronConfig struct {
	Schedule          string
	TimeZone          string
	ConcurrencyPolicy string
}

// CronValidationResult returns the validated and normalized cron job schedule settings.
type CronValidationResult struct {
	Schedule string

	TimeZone string
	Location *time.Location

	ConcurrencyPolicy      batchv1.ConcurrencyPolicy
	HasExplicitConcurrency bool
	HasExplicitTimeZone    bool
}

// ValidateCronConfig parses and validates a cron schedule, optional timezone, and concurrency policy.
//
// It rejects unsupported descriptors (such as @every), ensures timezone identifiers resolve to a
// known location, and constrains the concurrency policy to the options supported by Kubernetes.
func ValidateCronConfig(cfg CronConfig) (CronValidationResult, error) {
	schedule := strings.TrimSpace(cfg.Schedule)
	if schedule == "" {
		return CronValidationResult{}, ErrCronScheduleRequired
	}

	if strings.HasPrefix(strings.ToLower(schedule), "@every") {
		return CronValidationResult{}, ErrCronDescriptorUnsupported
	}

	if _, err := scheduleParser.Parse(schedule); err != nil {
		return CronValidationResult{}, fmt.Errorf("%w: %v", ErrCronScheduleInvalid, err)
	}

	res := CronValidationResult{Schedule: schedule, ConcurrencyPolicy: batchv1.AllowConcurrent}

	tz := strings.TrimSpace(cfg.TimeZone)
	if tz != "" {
		loc, err := time.LoadLocation(tz)
		if err != nil {
			return CronValidationResult{}, fmt.Errorf("%w: %v", ErrCronTimeZoneInvalid, err)
		}
		res.TimeZone = tz
		res.Location = loc
		res.HasExplicitTimeZone = true
	}

	policy := strings.TrimSpace(cfg.ConcurrencyPolicy)
	if policy != "" {
		switch batchv1.ConcurrencyPolicy(policy) {
		case batchv1.AllowConcurrent, batchv1.ForbidConcurrent, batchv1.ReplaceConcurrent:
			res.ConcurrencyPolicy = batchv1.ConcurrencyPolicy(policy)
			res.HasExplicitConcurrency = true
		default:
			return CronValidationResult{}, fmt.Errorf("%w: %s (allowed: %s, %s, %s)",
				ErrCronConcurrencyPolicyInvalid,
				policy,
				batchv1.AllowConcurrent,
				batchv1.ForbidConcurrent,
				batchv1.ReplaceConcurrent,
			)
		}
	}

	return res, nil
}
