package metrics

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	tenantLabelKey  = "paas.kubeop.io/tenant"
	projectLabelKey = "paas.kubeop.io/project"
	appLabelKey     = "paas.kubeop.io/app"
)

// MetricsServerProvider queries the Kubernetes metrics-server API for pod usage.
type MetricsServerProvider struct {
	client metricsclient.Interface
	reader client.Reader
	logger *zap.SugaredLogger
}

// NewMetricsServerProvider builds a provider backed by metrics-server.
func NewMetricsServerProvider(cfg *rest.Config, reader client.Reader, logger *zap.SugaredLogger) (*MetricsServerProvider, error) {
	metricsClient, err := metricsclient.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build metrics client: %w", err)
	}
	return &MetricsServerProvider{client: metricsClient, reader: reader, logger: logger}, nil
}

// CollectUsage sums pod metrics by tenant/project/app labels.
func (p *MetricsServerProvider) CollectUsage(ctx context.Context, window time.Time) ([]UsageSample, error) {
	if p.client == nil {
		return nil, fmt.Errorf("metrics client is not configured")
	}

	var namespaces corev1.NamespaceList
	if err := p.reader.List(ctx, &namespaces); err != nil {
		return nil, fmt.Errorf("list namespaces: %w", err)
	}

	samples := make(map[string]*UsageSample)
	for _, ns := range namespaces.Items {
		podMetrics, err := p.client.MetricsV1beta1().PodMetricses(ns.Name).List(ctx, metav1.ListOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil, fmt.Errorf("metrics API unavailable: %w", err)
			}
			return nil, fmt.Errorf("list pod metrics for namespace %s: %w", ns.Name, err)
		}

		for _, metric := range podMetrics.Items {
			tenant := firstNonEmpty(metric.Labels[tenantLabelKey], ns.Labels[tenantLabelKey])
			project := firstNonEmpty(metric.Labels[projectLabelKey], ns.Labels[projectLabelKey])
			app := metric.Labels[appLabelKey]
			if strings.TrimSpace(tenant) == "" || strings.TrimSpace(project) == "" || strings.TrimSpace(app) == "" {
				continue
			}

			key := fmt.Sprintf("%s/%s", metric.Namespace, app)
			sample := samples[key]
			if sample == nil {
				sample = &UsageSample{
					Tenant:           tenant,
					ProjectNamespace: metric.Namespace,
					Project:          project,
					AppNamespace:     metric.Namespace,
					App:              app,
					CPU:              resource.MustParse("0"),
					Memory:           resource.MustParse("0"),
					Storage:          resource.MustParse("0"),
					Egress:           resource.MustParse("0"),
					LBHours:          resource.MustParse("0"),
				}
				samples[key] = sample
			}

			for _, container := range metric.Containers {
				if cpu, ok := container.Usage[corev1.ResourceCPU]; ok {
					sample.CPU.Add(cpu)
				}
				if memory, ok := container.Usage[corev1.ResourceMemory]; ok {
					sample.Memory.Add(memory)
				}
			}
		}
	}

	out := make([]UsageSample, 0, len(samples))
	for _, sample := range samples {
		out = append(out, *sample)
	}

	if len(out) == 0 && p.logger != nil {
		p.logger.Debug("metrics-server returned no pod usage samples")
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
