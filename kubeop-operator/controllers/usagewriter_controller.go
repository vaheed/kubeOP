package controllers

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	appv1alpha1 "github.com/vaheed/kubeOP/kubeop-operator/apis/paas/v1alpha1"
	"github.com/vaheed/kubeOP/kubeop-operator/internal/metrics"
	"go.uber.org/zap"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	usageWindowFormat              = "2006-01-02T15"
	usageAggregationCompleteReason = "UsageAggregationComplete"
	usageAggregationFailedReason   = "UsageAggregationFailed"
)

type usageClock interface {
	Now() time.Time
}

type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// UsageWriterReconciler aggregates usage metrics into BillingUsage snapshots.
type UsageWriterReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Logger   *zap.SugaredLogger
	Source   metrics.Provider
	Clock    usageClock
	Interval time.Duration
}

// SetupWithManager wires the reconciler into the controller manager.
func (r *UsageWriterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Source == nil {
		return fmt.Errorf("usage writer requires a metrics provider")
	}
	if r.Clock == nil {
		r.Clock = realClock{}
	}
	if r.Interval <= 0 {
		r.Interval = 5 * time.Minute
	}
	return mgr.Add(r)
}

// Start enqueues reconcile requests on the configured interval.
func (r *UsageWriterReconciler) Start(ctx context.Context) error {
	logger := r.logger()
	logger.Infow("starting usage writer", "interval", r.Interval)
	if err := r.runOnce(ctx); err != nil {
		logger.Errorw("usage aggregation failed", "error", err)
	}
	ticker := time.NewTicker(r.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			logger.Infow("stopping usage writer")
			return nil
		case <-ticker.C:
			if err := r.runOnce(ctx); err != nil {
				logger.Errorw("usage aggregation failed", "error", err)
			}
		}
	}
}

func (r *UsageWriterReconciler) runOnce(ctx context.Context) error {
	_, err := r.Reconcile(ctx, ctrl.Request{})
	return err
}

// Reconcile aggregates metrics and writes BillingUsage snapshots.
func (r *UsageWriterReconciler) Reconcile(ctx context.Context, _ ctrl.Request) (ctrl.Result, error) {
	logger := r.logger()
	window := r.Clock.Now().UTC().Truncate(time.Hour)
	logger.Infow("collecting usage samples", "window", window.Format(usageWindowFormat))

	samples, err := r.Source.CollectUsage(ctx, window)
	if err != nil {
		logger.Errorw("failed to collect usage samples", "window", window.Format(usageWindowFormat), "error", err)
		if statusErr := r.updateConditionsOnError(ctx, window, err); statusErr != nil {
			err = errors.Join(err, statusErr)
		}
		return ctrl.Result{RequeueAfter: r.Interval}, err
	}

	tenantTotals := make(map[string]*usageAccumulator)
	projectTotals := make(map[string]*usageAccumulator)
	appTotals := make(map[string]*usageAccumulator)

	for _, sample := range samples {
		tenant := strings.TrimSpace(sample.Tenant)
		project := strings.TrimSpace(sample.Project)
		appName := strings.TrimSpace(sample.App)
		if tenant == "" || project == "" || appName == "" {
			continue
		}

		acc := tenantTotals[tenant]
		if acc == nil {
			acc = newUsageAccumulator()
			tenantTotals[tenant] = acc
		}
		acc.add(sample)

		projectKey := namespaceKey(sample.ProjectNamespace, project)
		pacc := projectTotals[projectKey]
		if pacc == nil {
			pacc = newUsageAccumulator()
			projectTotals[projectKey] = pacc
		}
		pacc.add(sample)

		appKey := namespaceKey(sample.AppNamespace, appName)
		aapp := appTotals[appKey]
		if aapp == nil {
			aapp = newUsageAccumulator()
			appTotals[appKey] = aapp
		}
		aapp.add(sample)
	}

	logger.Infow("usage samples aggregated", "window", window.Format(usageWindowFormat), "tenants", len(tenantTotals), "projects", len(projectTotals), "apps", len(appTotals))

	var errs []error

	var tenantList appv1alpha1.TenantList
	if err := r.List(ctx, &tenantList); err != nil {
		return ctrl.Result{RequeueAfter: r.Interval}, fmt.Errorf("list tenants: %w", err)
	}
	for i := range tenantList.Items {
		tenant := &tenantList.Items[i]
		totals := tenantTotals[tenant.Name]
		if totals == nil {
			totals = newUsageAccumulator()
		}
		if err := r.updateTenantStatus(ctx, tenant, totals, window, nil); err != nil {
			errs = append(errs, err)
		}
		if err := r.upsertBillingUsage(ctx, appv1alpha1.BillingSubjectTypeTenant, tenant.Name, totals, window, nil); err != nil {
			errs = append(errs, err)
		}
	}

	var projectList appv1alpha1.ProjectList
	if err := r.List(ctx, &projectList); err != nil {
		return ctrl.Result{RequeueAfter: r.Interval}, fmt.Errorf("list projects: %w", err)
	}
	for i := range projectList.Items {
		project := &projectList.Items[i]
		key := namespaceKey(project.Namespace, project.Name)
		totals := projectTotals[key]
		if totals == nil {
			totals = newUsageAccumulator()
		}
		if err := r.updateProjectStatus(ctx, project, totals, window, nil); err != nil {
			errs = append(errs, err)
		}
		if err := r.upsertBillingUsage(ctx, appv1alpha1.BillingSubjectTypeProject, key, totals, window, nil); err != nil {
			errs = append(errs, err)
		}
	}

	var appList appv1alpha1.AppList
	if err := r.List(ctx, &appList); err != nil {
		return ctrl.Result{RequeueAfter: r.Interval}, fmt.Errorf("list apps: %w", err)
	}
	for i := range appList.Items {
		app := &appList.Items[i]
		key := namespaceKey(app.Namespace, app.Name)
		totals := appTotals[key]
		if totals == nil {
			totals = newUsageAccumulator()
		}
		if err := r.upsertBillingUsage(ctx, appv1alpha1.BillingSubjectTypeApp, key, totals, window, nil); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return ctrl.Result{RequeueAfter: r.Interval}, errors.Join(errs...)
	}
	logger.Infow("usage snapshot reconciled", "window", window.Format(usageWindowFormat), "tenants", len(tenantList.Items), "projects", len(projectList.Items), "apps", len(appList.Items))
	return ctrl.Result{RequeueAfter: r.Interval}, nil
}

func (r *UsageWriterReconciler) updateConditionsOnError(ctx context.Context, window time.Time, aggregationErr error) error {
	var errs []error

	var tenantList appv1alpha1.TenantList
	if err := r.List(ctx, &tenantList); err != nil {
		errs = append(errs, fmt.Errorf("list tenants: %w", err))
	} else {
		for i := range tenantList.Items {
			tenant := &tenantList.Items[i]
			original := tenant.DeepCopy()
			setUsageConditions(&tenant.Status.Conditions, window, aggregationErr)
			if err := r.Status().Patch(ctx, tenant, client.MergeFrom(original)); err != nil {
				errs = append(errs, fmt.Errorf("patch tenant status %s: %w", tenant.Name, err))
			}
		}
	}

	var projectList appv1alpha1.ProjectList
	if err := r.List(ctx, &projectList); err != nil {
		errs = append(errs, fmt.Errorf("list projects: %w", err))
	} else {
		for i := range projectList.Items {
			project := &projectList.Items[i]
			original := project.DeepCopy()
			setUsageConditions(&project.Status.Conditions, window, aggregationErr)
			if err := r.Status().Patch(ctx, project, client.MergeFrom(original)); err != nil {
				errs = append(errs, fmt.Errorf("patch project status %s/%s: %w", project.Namespace, project.Name, err))
			}
		}
	}

	return errors.Join(errs...)
}

func (r *UsageWriterReconciler) updateTenantStatus(ctx context.Context, tenant *appv1alpha1.Tenant, totals *usageAccumulator, window time.Time, aggregationErr error) error {
	original := tenant.DeepCopy()
	tenant.Status.Usage = &appv1alpha1.TenantUsageStatus{
		CPU:     quantityPtr(totals.CPU),
		Memory:  quantityPtr(totals.Memory),
		Storage: quantityPtr(totals.Storage),
		Egress:  quantityPtr(totals.Egress),
		LBHours: quantityPtr(totals.LBHours),
	}
	setUsageConditions(&tenant.Status.Conditions, window, aggregationErr)
	if err := r.Status().Patch(ctx, tenant, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch tenant status %s: %w", tenant.Name, err)
	}
	return nil
}

func (r *UsageWriterReconciler) updateProjectStatus(ctx context.Context, project *appv1alpha1.Project, totals *usageAccumulator, window time.Time, aggregationErr error) error {
	original := project.DeepCopy()
	project.Status.Usage = &appv1alpha1.ProjectUsageStatus{
		CPU:     quantityPtr(totals.CPU),
		Memory:  quantityPtr(totals.Memory),
		Storage: quantityPtr(totals.Storage),
		Egress:  quantityPtr(totals.Egress),
		LBHours: quantityPtr(totals.LBHours),
	}
	setUsageConditions(&project.Status.Conditions, window, aggregationErr)
	if err := r.Status().Patch(ctx, project, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch project status %s/%s: %w", project.Namespace, project.Name, err)
	}
	return nil
}

func (r *UsageWriterReconciler) upsertBillingUsage(ctx context.Context, subjectType appv1alpha1.BillingSubjectType, subjectRef string, totals *usageAccumulator, window time.Time, aggregationErr error) error {
	name := buildBillingUsageName(subjectType, subjectRef, window)
	usage := &appv1alpha1.BillingUsage{ObjectMeta: metav1.ObjectMeta{Name: name}}
	_, err := controllerutil.CreateOrUpdate(ctx, r.Client, usage, func() error {
		usage.Spec.SubjectType = subjectType
		usage.Spec.SubjectRef = subjectRef
		usage.Spec.Window = window.Format(usageWindowFormat)
		if usage.Spec.Meters == nil {
			usage.Spec.Meters = map[string]string{}
		}
		usage.Spec.Meters["cpu"] = totals.CPU.String()
		usage.Spec.Meters["memory"] = totals.Memory.String()
		usage.Spec.Meters["storage"] = totals.Storage.String()
		usage.Spec.Meters["egress"] = totals.Egress.String()
		usage.Spec.Meters["lbHours"] = totals.LBHours.String()
		return nil
	})
	if err != nil {
		return fmt.Errorf("sync billing usage %s/%s: %w", subjectType, subjectRef, err)
	}
	original := usage.DeepCopy()
	setUsageConditions(&usage.Status.Conditions, window, aggregationErr)
	if err := r.Status().Patch(ctx, usage, client.MergeFrom(original)); err != nil {
		return fmt.Errorf("patch billing usage status %s/%s: %w", subjectType, subjectRef, err)
	}
	return nil
}

func setUsageConditions(conditions *[]metav1.Condition, window time.Time, aggregationErr error) {
	windowMsg := fmt.Sprintf("Usage window %s", window.Format(time.RFC3339))
	if aggregationErr != nil {
		msg := fmt.Sprintf("%s: %v", windowMsg, aggregationErr)
		apimeta.SetStatusCondition(conditions, metav1.Condition{Type: "Reconciling", Status: metav1.ConditionFalse, Reason: usageAggregationFailedReason, Message: msg})
		apimeta.SetStatusCondition(conditions, metav1.Condition{Type: "Ready", Status: metav1.ConditionFalse, Reason: usageAggregationFailedReason, Message: msg})
		apimeta.SetStatusCondition(conditions, metav1.Condition{Type: "Degraded", Status: metav1.ConditionTrue, Reason: usageAggregationFailedReason, Message: msg})
		return
	}
	message := fmt.Sprintf("%s updated", windowMsg)
	apimeta.SetStatusCondition(conditions, metav1.Condition{Type: "Reconciling", Status: metav1.ConditionFalse, Reason: usageAggregationCompleteReason, Message: message})
	apimeta.SetStatusCondition(conditions, metav1.Condition{Type: "Ready", Status: metav1.ConditionTrue, Reason: usageAggregationCompleteReason, Message: message})
	apimeta.SetStatusCondition(conditions, metav1.Condition{Type: "Degraded", Status: metav1.ConditionFalse, Reason: usageAggregationCompleteReason, Message: "Usage aggregation healthy"})
}

func quantityPtr(q resource.Quantity) *resource.Quantity {
	v := q
	return &v
}

func namespaceKey(ns, name string) string {
	if strings.TrimSpace(ns) == "" {
		return name
	}
	return fmt.Sprintf("%s/%s", ns, name)
}

func buildBillingUsageName(subjectType appv1alpha1.BillingSubjectType, subjectRef string, window time.Time) string {
	base := fmt.Sprintf("%s-%s-%s", strings.ToLower(string(subjectType)), sanitizeName(subjectRef), sanitizeWindow(window))
	if len(base) <= validation.DNS1123LabelMaxLength {
		return base
	}
	hash := sha1.Sum([]byte(base))
	suffix := hex.EncodeToString(hash[:4])
	trimmedLength := validation.DNS1123LabelMaxLength - len(suffix) - 1
	if trimmedLength < 1 {
		trimmedLength = 1
	}
	prefix := sanitizeName(base[:trimmedLength])
	if prefix == "" {
		prefix = "usage"
	}
	return fmt.Sprintf("%s-%s", prefix, suffix)
}

func sanitizeName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer("/", "-", ":", "-", "_", "-", ".", "-")
	value = replacer.Replace(value)
	builder := strings.Builder{}
	lastDash := false
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				builder.WriteRune('-')
				lastDash = true
			}
		}
	}
	sanitized := strings.Trim(builder.String(), "-")
	if sanitized == "" {
		sanitized = "subject"
	}
	return sanitized
}

func sanitizeWindow(window time.Time) string {
	formatted := window.Format(usageWindowFormat)
	formatted = strings.ReplaceAll(formatted, ":", "-")
	return strings.ToLower(formatted)
}

type usageAccumulator struct {
	CPU     resource.Quantity
	Memory  resource.Quantity
	Storage resource.Quantity
	Egress  resource.Quantity
	LBHours resource.Quantity
}

func newUsageAccumulator() *usageAccumulator {
	return &usageAccumulator{
		CPU:     resource.MustParse("0"),
		Memory:  resource.MustParse("0"),
		Storage: resource.MustParse("0"),
		Egress:  resource.MustParse("0"),
		LBHours: resource.MustParse("0"),
	}
}

func (a *usageAccumulator) add(sample metrics.UsageSample) {
	if sample.CPU.Sign() != 0 {
		a.CPU.Add(sample.CPU)
	}
	if sample.Memory.Sign() != 0 {
		a.Memory.Add(sample.Memory)
	}
	if sample.Storage.Sign() != 0 {
		a.Storage.Add(sample.Storage)
	}
	if sample.Egress.Sign() != 0 {
		a.Egress.Add(sample.Egress)
	}
	if sample.LBHours.Sign() != 0 {
		a.LBHours.Add(sample.LBHours)
	}
}

func (r *UsageWriterReconciler) logger() *zap.SugaredLogger {
	if r.Logger != nil {
		return r.Logger
	}
	return zap.NewNop().Sugar()
}
