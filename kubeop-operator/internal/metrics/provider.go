package metrics

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
)

// UsageSample represents metered usage for a single application workload.
type UsageSample struct {
	Tenant           string
	ProjectNamespace string
	Project          string
	AppNamespace     string
	App              string

	CPU     resource.Quantity
	Memory  resource.Quantity
	Storage resource.Quantity
	Egress  resource.Quantity
	LBHours resource.Quantity
}

// Provider reports usage snapshots for billing windows.
type Provider interface {
	CollectUsage(ctx context.Context, window time.Time) ([]UsageSample, error)
}
