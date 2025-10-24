package webhooks

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation/field"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	appv1alpha1 "github.com/vaheed/kubeOP/kubeop-operator/apis/paas/v1alpha1"
)

const (
	labelTenant                      = "paas.kubeop.io/tenant"
	labelProject                     = "paas.kubeop.io/project"
	labelApp                         = "paas.kubeop.io/app"
	annotationRunAsRootJustification = "paas.kubeop.io/run-as-root-justification"
)

// Setup registers all admission webhooks with the manager.
func Setup(mgr ctrl.Manager, logger *zap.SugaredLogger) error {
	appWebhook := &AppWebhook{
		client: mgr.GetClient(),
		logger: logger.Named("app"),
	}
	if err := appWebhook.SetupWithManager(mgr); err != nil {
		return err
	}
	jobWebhook := &JobWebhook{
		logger: logger.Named("job"),
	}
	return jobWebhook.SetupWithManager(mgr)
}

// AppWebhook validates tenant isolation for App resources.
type AppWebhook struct {
	client client.Client
	logger *zap.SugaredLogger
}

var _ webhook.CustomValidator = (*AppWebhook)(nil)
var _ webhook.CustomDefaulter = (*AppWebhook)(nil)

// JobWebhook enforces hardened pod security defaults for Jobs.
type JobWebhook struct {
	logger *zap.SugaredLogger
}

var _ webhook.CustomValidator = (*JobWebhook)(nil)
var _ webhook.CustomDefaulter = (*JobWebhook)(nil)

// SetupWithManager wires the webhook into the controller manager.
func (w *AppWebhook) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&appv1alpha1.App{}).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

// Default initialises metadata fields to avoid nil map panics.
func (w *AppWebhook) Default(ctx context.Context, obj runtime.Object) error {
	app, ok := obj.(*appv1alpha1.App)
	if !ok {
		return fmt.Errorf("expected App, got %T", obj)
	}
	if app.Labels == nil {
		app.Labels = map[string]string{}
	}
	return nil
}

// ValidateCreate enforces tenant labelling and reference isolation.
func (w *AppWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (webhook.Warnings, error) {
	app, ok := obj.(*appv1alpha1.App)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected App, got %T", obj))
	}
	if errs := w.validateApp(ctx, nil, app); len(errs) > 0 {
		return nil, apierrors.NewInvalid(appv1alpha1.GroupVersion.WithKind("App").GroupKind(), app.Name, errs)
	}
	return nil, nil
}

// ValidateUpdate ensures tenant metadata cannot be mutated and references stay scoped.
func (w *AppWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (webhook.Warnings, error) {
	newApp, ok := newObj.(*appv1alpha1.App)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected App, got %T", newObj))
	}
	oldApp, ok := oldObj.(*appv1alpha1.App)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected App, got %T", oldObj))
	}
	if errs := w.validateApp(ctx, oldApp, newApp); len(errs) > 0 {
		return nil, apierrors.NewInvalid(appv1alpha1.GroupVersion.WithKind("App").GroupKind(), newApp.Name, errs)
	}
	return nil, nil
}

// ValidateDelete performs no-op validation for deletions.
func (w *AppWebhook) ValidateDelete(ctx context.Context, obj runtime.Object) (webhook.Warnings, error) {
	return nil, nil
}

func (w *AppWebhook) validateApp(ctx context.Context, oldApp, newApp *appv1alpha1.App) field.ErrorList {
	labels := newApp.GetLabels()
	var errs field.ErrorList
	required := map[string]string{
		labelTenant:  "tenant",
		labelProject: "project",
		labelApp:     "app",
	}
	for key, description := range required {
		if labels[key] == "" {
			errs = append(errs, field.Required(field.NewPath("metadata", "labels").Key(key), fmt.Sprintf("%s label is required for tenant isolation", description)))
		}
	}

	if oldApp != nil {
		if oldTenant := oldApp.GetLabels()[labelTenant]; oldTenant != "" && labels[labelTenant] != oldTenant {
			errs = append(errs, field.Forbidden(field.NewPath("metadata", "labels").Key(labelTenant), "tenant label is immutable"))
		}
	}

	tenant := labels[labelTenant]
	if tenant == "" {
		return errs
	}

	errs = append(errs, w.validateConfigRefs(ctx, newApp, tenant)...)
	errs = append(errs, w.validateSecretRefs(ctx, newApp, tenant)...)
	errs = append(errs, w.validateIngressRefs(ctx, newApp, tenant)...)

	return errs
}

func (w *AppWebhook) validateConfigRefs(ctx context.Context, app *appv1alpha1.App, tenant string) field.ErrorList {
	var errs field.ErrorList
	if len(app.Spec.ValuesRefs) == 0 {
		return errs
	}
	path := field.NewPath("spec", "valuesRefs")
	for idx, name := range app.Spec.ValuesRefs {
		var ref appv1alpha1.ConfigRef
		key := types.NamespacedName{Namespace: app.Namespace, Name: name}
		if err := w.client.Get(ctx, key, &ref); err != nil {
			if apierrors.IsNotFound(err) {
				errs = append(errs, field.Invalid(path.Index(idx), name, "referenced ConfigRef does not exist"))
				continue
			}
			w.logger.Errorw("Failed to fetch ConfigRef", "name", key, "error", err)
			errs = append(errs, field.InternalError(path.Index(idx), fmt.Errorf("unable to verify ConfigRef %q", name)))
			continue
		}
		if refTenant := ref.GetLabels()[labelTenant]; refTenant != tenant {
			message := fmt.Sprintf("ConfigRef %q belongs to tenant %q", name, refTenant)
			errs = append(errs, field.Forbidden(path.Index(idx), message))
		}
	}
	return errs
}

func (w *AppWebhook) validateSecretRefs(ctx context.Context, app *appv1alpha1.App, tenant string) field.ErrorList {
	var errs field.ErrorList
	if len(app.Spec.SecretsRefs) == 0 {
		return errs
	}
	path := field.NewPath("spec", "secretsRefs")
	for idx, name := range app.Spec.SecretsRefs {
		var ref appv1alpha1.SecretRef
		key := types.NamespacedName{Namespace: app.Namespace, Name: name}
		if err := w.client.Get(ctx, key, &ref); err != nil {
			if apierrors.IsNotFound(err) {
				errs = append(errs, field.Invalid(path.Index(idx), name, "referenced SecretRef does not exist"))
				continue
			}
			w.logger.Errorw("Failed to fetch SecretRef", "name", key, "error", err)
			errs = append(errs, field.InternalError(path.Index(idx), fmt.Errorf("unable to verify SecretRef %q", name)))
			continue
		}
		if refTenant := ref.GetLabels()[labelTenant]; refTenant != tenant {
			message := fmt.Sprintf("SecretRef %q belongs to tenant %q", name, refTenant)
			errs = append(errs, field.Forbidden(path.Index(idx), message))
		}
	}
	return errs
}

func (w *AppWebhook) validateIngressRefs(ctx context.Context, app *appv1alpha1.App, tenant string) field.ErrorList {
	var errs field.ErrorList
	if len(app.Spec.IngressRefs) == 0 {
		return errs
	}
	path := field.NewPath("spec", "ingressRefs")
	for idx, name := range app.Spec.IngressRefs {
		var ref appv1alpha1.IngressRoute
		key := types.NamespacedName{Namespace: app.Namespace, Name: name}
		if err := w.client.Get(ctx, key, &ref); err != nil {
			if apierrors.IsNotFound(err) {
				errs = append(errs, field.Invalid(path.Index(idx), name, "referenced IngressRoute does not exist"))
				continue
			}
			w.logger.Errorw("Failed to fetch IngressRoute", "name", key, "error", err)
			errs = append(errs, field.InternalError(path.Index(idx), fmt.Errorf("unable to verify IngressRoute %q", name)))
			continue
		}
		if refTenant := ref.GetLabels()[labelTenant]; refTenant != tenant {
			message := fmt.Sprintf("IngressRoute %q belongs to tenant %q", name, refTenant)
			errs = append(errs, field.Forbidden(path.Index(idx), message))
		}
	}
	return errs
}

// SetupWithManager wires the job webhook into the controller manager.
func (w *JobWebhook) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&appv1alpha1.Job{}).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

// Default initialises metadata for Job resources.
func (w *JobWebhook) Default(_ context.Context, obj runtime.Object) error {
	job, ok := obj.(*appv1alpha1.Job)
	if !ok {
		return fmt.Errorf("expected Job, got %T", obj)
	}
	if job.Labels == nil {
		job.Labels = map[string]string{}
	}
	if job.Annotations == nil {
		job.Annotations = map[string]string{}
	}
	return nil
}

// ValidateCreate enforces pod security restrictions for tenant Jobs.
func (w *JobWebhook) ValidateCreate(_ context.Context, obj runtime.Object) (webhook.Warnings, error) {
	job, ok := obj.(*appv1alpha1.Job)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected Job, got %T", obj))
	}
	if errs := w.validateJob(job); len(errs) > 0 {
		return nil, apierrors.NewInvalid(appv1alpha1.GroupVersion.WithKind("Job").GroupKind(), job.Name, errs)
	}
	return nil, nil
}

// ValidateUpdate reuses create-time enforcement for Job updates.
func (w *JobWebhook) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (webhook.Warnings, error) {
	job, ok := newObj.(*appv1alpha1.Job)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected Job, got %T", newObj))
	}
	if errs := w.validateJob(job); len(errs) > 0 {
		return nil, apierrors.NewInvalid(appv1alpha1.GroupVersion.WithKind("Job").GroupKind(), job.Name, errs)
	}
	return nil, nil
}

// ValidateDelete performs no-op validation for Job deletions.
func (w *JobWebhook) ValidateDelete(context.Context, runtime.Object) (webhook.Warnings, error) {
	return nil, nil
}

func (w *JobWebhook) validateJob(job *appv1alpha1.Job) field.ErrorList {
	var errs field.ErrorList
	podSpec := job.Spec.Template.Spec
	podPath := field.NewPath("spec", "template", "spec")
	if podSpec.HostNetwork {
		errs = append(errs, field.Forbidden(podPath.Child("hostNetwork"), "hostNetwork is not allowed for tenant workloads"))
	}
	if podSpec.HostPID {
		errs = append(errs, field.Forbidden(podPath.Child("hostPID"), "hostPID is not allowed for tenant workloads"))
	}
	if podSpec.HostIPC {
		errs = append(errs, field.Forbidden(podPath.Child("hostIPC"), "hostIPC is not allowed for tenant workloads"))
	}
	for idx, vol := range podSpec.Volumes {
		if vol.HostPath != nil {
			errs = append(errs, field.Forbidden(podPath.Child("volumes").Index(idx), "hostPath volumes are not permitted"))
		}
	}

	for idx, c := range podSpec.InitContainers {
		if containsSysAdmin(c.SecurityContext) {
			errs = append(errs, field.Forbidden(containerCapabilitiesPath(podPath, "initContainers", idx), "CAP_SYS_ADMIN capability is not permitted"))
		}
	}
	for idx, c := range podSpec.Containers {
		if containsSysAdmin(c.SecurityContext) {
			errs = append(errs, field.Forbidden(containerCapabilitiesPath(podPath, "containers", idx), "CAP_SYS_ADMIN capability is not permitted"))
		}
	}

	if requiresRootJustification(podSpec) {
		justification := strings.TrimSpace(job.Annotations[annotationRunAsRootJustification])
		if justification == "" {
			errs = append(errs, field.Required(field.NewPath("metadata", "annotations").Key(annotationRunAsRootJustification), "runAsRoot workloads must include a justification"))
		}
	}

	return errs
}

func containsSysAdmin(sc *corev1.SecurityContext) bool {
	if sc == nil || sc.Capabilities == nil {
		return false
	}
	for _, cap := range sc.Capabilities.Add {
		if strings.EqualFold(string(cap), "SYS_ADMIN") {
			return true
		}
	}
	return false
}

func containerCapabilitiesPath(base *field.Path, list string, idx int) *field.Path {
	return base.Child(list).Index(idx).Child("securityContext").Child("capabilities")
}

func requiresRootJustification(podSpec corev1.PodSpec) bool {
	if allowsRoot(podSpec.SecurityContext) {
		return true
	}
	for _, c := range podSpec.InitContainers {
		if allowsRootWithFallback(c.SecurityContext, podSpec.SecurityContext) {
			return true
		}
	}
	for _, c := range podSpec.Containers {
		if allowsRootWithFallback(c.SecurityContext, podSpec.SecurityContext) {
			return true
		}
	}
	return false
}

func allowsRoot(sc *corev1.PodSecurityContext) bool {
	if sc == nil {
		return false
	}
	if sc.RunAsNonRoot != nil && !*sc.RunAsNonRoot {
		return true
	}
	if sc.RunAsUser != nil && *sc.RunAsUser == 0 {
		return true
	}
	return false
}

func allowsRootWithFallback(containerSC *corev1.SecurityContext, podSC *corev1.PodSecurityContext) bool {
	if containerSC != nil {
		if containerSC.RunAsNonRoot != nil && !*containerSC.RunAsNonRoot {
			return true
		}
		if containerSC.RunAsUser != nil && *containerSC.RunAsUser == 0 {
			return true
		}
	}
	return allowsRoot(podSC)
}
