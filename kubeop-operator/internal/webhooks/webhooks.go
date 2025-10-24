package webhooks

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
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

var requiredLabelKeys = []string{labelTenant, labelProject, labelApp}

func defaultMetadata(obj metav1.Object) {
	if obj.GetLabels() == nil {
		obj.SetLabels(map[string]string{})
	}
	if obj.GetAnnotations() == nil {
		obj.SetAnnotations(map[string]string{})
	}
}

func validateRequiredLabels(obj metav1.Object) field.ErrorList {
	var errs field.ErrorList
	path := field.NewPath("metadata", "labels")
	labels := obj.GetLabels()
	for _, key := range requiredLabelKeys {
		if strings.TrimSpace(labels[key]) == "" {
			errs = append(errs, field.Required(path.Key(key), fmt.Sprintf("%s label is required for tenant isolation", strings.TrimPrefix(key, "paas.kubeop.io/"))))
		}
	}
	return errs
}

func validateTenantImmutability(oldObj, newObj metav1.Object) field.ErrorList {
	var errs field.ErrorList
	if oldObj == nil || newObj == nil {
		return errs
	}
	oldTenant := strings.TrimSpace(oldObj.GetLabels()[labelTenant])
	newTenant := strings.TrimSpace(newObj.GetLabels()[labelTenant])
	if oldTenant != "" && newTenant != oldTenant {
		errs = append(errs, field.Forbidden(field.NewPath("metadata", "labels").Key(labelTenant), "tenant label is immutable"))
	}
	return errs
}

// Setup registers all admission webhooks with the manager.
func Setup(mgr ctrl.Manager, logger *zap.SugaredLogger) error {
	appWebhook := &AppWebhook{
		client: mgr.GetClient(),
		logger: logger.Named("app"),
	}
	if err := appWebhook.SetupWithManager(mgr); err != nil {
		return err
	}

	projectWebhook := &ProjectWebhook{
		client: mgr.GetClient(),
		logger: logger.Named("project"),
	}
	if err := projectWebhook.SetupWithManager(mgr); err != nil {
		return err
	}

	appReleaseWebhook := &AppReleaseWebhook{
		client: mgr.GetClient(),
		logger: logger.Named("apprelease"),
	}
	if err := appReleaseWebhook.SetupWithManager(mgr); err != nil {
		return err
	}

	serviceBindingWebhook := &ServiceBindingWebhook{
		client: mgr.GetClient(),
		logger: logger.Named("servicebinding"),
	}
	if err := serviceBindingWebhook.SetupWithManager(mgr); err != nil {
		return err
	}

	bucketWebhook := &BucketWebhook{
		client: mgr.GetClient(),
		logger: logger.Named("bucket"),
	}
	if err := bucketWebhook.SetupWithManager(mgr); err != nil {
		return err
	}

	genericRegistrations := []struct {
		obj  client.Object
		gvk  schema.GroupVersionKind
		name string
	}{
		{&appv1alpha1.ConfigRef{}, appv1alpha1.GroupVersion.WithKind("ConfigRef"), "configref"},
		{&appv1alpha1.SecretRef{}, appv1alpha1.GroupVersion.WithKind("SecretRef"), "secretref"},
		{&appv1alpha1.IngressRoute{}, appv1alpha1.GroupVersion.WithKind("IngressRoute"), "ingressroute"},
		{&appv1alpha1.CacheInstance{}, appv1alpha1.GroupVersion.WithKind("CacheInstance"), "cacheinstance"},
		{&appv1alpha1.DatabaseInstance{}, appv1alpha1.GroupVersion.WithKind("DatabaseInstance"), "databaseinstance"},
		{&appv1alpha1.QueueInstance{}, appv1alpha1.GroupVersion.WithKind("QueueInstance"), "queueinstance"},
		{&appv1alpha1.Bucket{}, appv1alpha1.GroupVersion.WithKind("Bucket"), "bucketlabels"},
		{&appv1alpha1.BucketPolicy{}, appv1alpha1.GroupVersion.WithKind("BucketPolicy"), "bucketpolicy"},
	}
	for _, registration := range genericRegistrations {
		if err := registerLabelGuard(mgr, registration.obj, registration.gvk, logger.Named(registration.name)); err != nil {
			return err
		}
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

// LabelGuardWebhook enforces required metadata on namespace-scoped resources without bespoke logic.
type LabelGuardWebhook struct {
	gvk    schema.GroupVersionKind
	logger *zap.SugaredLogger
}

var _ webhook.CustomValidator = (*LabelGuardWebhook)(nil)
var _ webhook.CustomDefaulter = (*LabelGuardWebhook)(nil)

func registerLabelGuard(mgr ctrl.Manager, obj client.Object, gvk schema.GroupVersionKind, logger *zap.SugaredLogger) error {
	webhook := &LabelGuardWebhook{gvk: gvk, logger: logger}
	return ctrl.NewWebhookManagedBy(mgr).
		For(obj).
		WithDefaulter(webhook).
		WithValidator(webhook).
		Complete()
}

// Default ensures metadata maps are initialised before mutation.
func (w *LabelGuardWebhook) Default(_ context.Context, obj runtime.Object) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return fmt.Errorf("metadata accessor: %w", err)
	}
	defaultMetadata(accessor)
	return nil
}

// ValidateCreate enforces required metadata labels on create.
func (w *LabelGuardWebhook) ValidateCreate(_ context.Context, obj runtime.Object) (webhook.Warnings, error) {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected %s, got %T", w.gvk.Kind, obj))
	}
	if errs := validateRequiredLabels(accessor); len(errs) > 0 {
		return nil, apierrors.NewInvalid(w.gvk.GroupKind(), accessor.GetName(), errs)
	}
	return nil, nil
}

// ValidateUpdate reuses create-time checks and forbids tenant label drift.
func (w *LabelGuardWebhook) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (webhook.Warnings, error) {
	newAccessor, err := meta.Accessor(newObj)
	if err != nil {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected %s, got %T", w.gvk.Kind, newObj))
	}
	oldAccessor, err := meta.Accessor(oldObj)
	if err != nil {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected %s, got %T", w.gvk.Kind, oldObj))
	}
	errs := validateRequiredLabels(newAccessor)
	errs = append(errs, validateTenantImmutability(oldAccessor, newAccessor)...)
	if len(errs) > 0 {
		return nil, apierrors.NewInvalid(w.gvk.GroupKind(), newAccessor.GetName(), errs)
	}
	return nil, nil
}

// ValidateDelete performs no-op validation for deletes.
func (w *LabelGuardWebhook) ValidateDelete(context.Context, runtime.Object) (webhook.Warnings, error) {
	return nil, nil
}

// JobWebhook enforces hardened pod security defaults for Jobs.
type JobWebhook struct {
	logger *zap.SugaredLogger
}

var _ webhook.CustomValidator = (*JobWebhook)(nil)
var _ webhook.CustomDefaulter = (*JobWebhook)(nil)

// ProjectWebhook validates tenant isolation for Project resources.
type ProjectWebhook struct {
	client client.Client
	logger *zap.SugaredLogger
}

var _ webhook.CustomValidator = (*ProjectWebhook)(nil)
var _ webhook.CustomDefaulter = (*ProjectWebhook)(nil)

// AppReleaseWebhook guards cross-tenant relationships between AppRelease and App.
type AppReleaseWebhook struct {
	client client.Client
	logger *zap.SugaredLogger
}

var _ webhook.CustomValidator = (*AppReleaseWebhook)(nil)
var _ webhook.CustomDefaulter = (*AppReleaseWebhook)(nil)

// ServiceBindingWebhook validates consumer and provider references.
type ServiceBindingWebhook struct {
	client client.Client
	logger *zap.SugaredLogger
}

var _ webhook.CustomValidator = (*ServiceBindingWebhook)(nil)
var _ webhook.CustomDefaulter = (*ServiceBindingWebhook)(nil)

// BucketWebhook validates policy references for Buckets.
type BucketWebhook struct {
	client client.Client
	logger *zap.SugaredLogger
}

var _ webhook.CustomValidator = (*BucketWebhook)(nil)
var _ webhook.CustomDefaulter = (*BucketWebhook)(nil)

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
	defaultMetadata(app)
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
	errs = append(errs, validateRequiredLabels(newApp)...)

	if oldApp != nil {
		errs = append(errs, validateTenantImmutability(oldApp, newApp)...)
	}

	tenant := labels[labelTenant]
	if tenant == "" {
		return errs
	}

	errs = append(errs, w.validateConfigRefs(ctx, newApp, tenant)...)
	errs = append(errs, w.validateSecretRefs(ctx, newApp, tenant)...)
	errs = append(errs, w.validateIngressRefs(ctx, newApp, tenant)...)
	project, projectErrs := w.projectForApp(ctx, newApp, tenant)
	errs = append(errs, projectErrs...)
	errs = append(errs, w.validateServicePolicy(ctx, newApp, project)...)

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

func (w *ProjectWebhook) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&appv1alpha1.Project{}).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

func (w *ProjectWebhook) Default(_ context.Context, obj runtime.Object) error {
	project, ok := obj.(*appv1alpha1.Project)
	if !ok {
		return fmt.Errorf("expected Project, got %T", obj)
	}
	defaultMetadata(project)
	return nil
}

func (w *ProjectWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (webhook.Warnings, error) {
	project, ok := obj.(*appv1alpha1.Project)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected Project, got %T", obj))
	}
	if errs := w.validateProject(ctx, nil, project); len(errs) > 0 {
		return nil, apierrors.NewInvalid(appv1alpha1.GroupVersion.WithKind("Project").GroupKind(), project.Name, errs)
	}
	return nil, nil
}

func (w *ProjectWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (webhook.Warnings, error) {
	newProject, ok := newObj.(*appv1alpha1.Project)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected Project, got %T", newObj))
	}
	oldProject, ok := oldObj.(*appv1alpha1.Project)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected Project, got %T", oldObj))
	}
	if errs := w.validateProject(ctx, oldProject, newProject); len(errs) > 0 {
		return nil, apierrors.NewInvalid(appv1alpha1.GroupVersion.WithKind("Project").GroupKind(), newProject.Name, errs)
	}
	return nil, nil
}

func (w *ProjectWebhook) ValidateDelete(context.Context, runtime.Object) (webhook.Warnings, error) {
	return nil, nil
}

func (w *ProjectWebhook) validateProject(_ context.Context, oldProject, newProject *appv1alpha1.Project) field.ErrorList {
	errs := validateRequiredLabels(newProject)
	if oldProject != nil {
		errs = append(errs, validateTenantImmutability(oldProject, newProject)...)
	}
	labels := newProject.GetLabels()
	tenant := strings.TrimSpace(labels[labelTenant])
	if tenant != "" && newProject.Spec.TenantRef != "" && tenant != newProject.Spec.TenantRef {
		errs = append(errs, field.Invalid(field.NewPath("spec", "tenantRef"), newProject.Spec.TenantRef, "tenantRef must match metadata tenant label"))
	}
	if projectLabel := strings.TrimSpace(labels[labelProject]); projectLabel != "" && projectLabel != newProject.Name {
		errs = append(errs, field.Invalid(field.NewPath("metadata", "labels").Key(labelProject), projectLabel, "project label must equal resource name"))
	}
	if oldProject != nil && oldProject.Spec.TenantRef != newProject.Spec.TenantRef {
		errs = append(errs, field.Forbidden(field.NewPath("spec", "tenantRef"), "tenantRef is immutable"))
	}
	return errs
}

func (w *AppReleaseWebhook) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&appv1alpha1.AppRelease{}).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

func (w *AppReleaseWebhook) Default(_ context.Context, obj runtime.Object) error {
	release, ok := obj.(*appv1alpha1.AppRelease)
	if !ok {
		return fmt.Errorf("expected AppRelease, got %T", obj)
	}
	defaultMetadata(release)
	return nil
}

func (w *AppReleaseWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (webhook.Warnings, error) {
	release, ok := obj.(*appv1alpha1.AppRelease)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected AppRelease, got %T", obj))
	}
	if errs := w.validateRelease(ctx, nil, release); len(errs) > 0 {
		return nil, apierrors.NewInvalid(appv1alpha1.GroupVersion.WithKind("AppRelease").GroupKind(), release.Name, errs)
	}
	return nil, nil
}

func (w *AppReleaseWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (webhook.Warnings, error) {
	newRelease, ok := newObj.(*appv1alpha1.AppRelease)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected AppRelease, got %T", newObj))
	}
	oldRelease, ok := oldObj.(*appv1alpha1.AppRelease)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected AppRelease, got %T", oldObj))
	}
	if errs := w.validateRelease(ctx, oldRelease, newRelease); len(errs) > 0 {
		return nil, apierrors.NewInvalid(appv1alpha1.GroupVersion.WithKind("AppRelease").GroupKind(), newRelease.Name, errs)
	}
	return nil, nil
}

func (w *AppReleaseWebhook) ValidateDelete(context.Context, runtime.Object) (webhook.Warnings, error) {
	return nil, nil
}

func (w *AppReleaseWebhook) validateRelease(ctx context.Context, oldRelease, newRelease *appv1alpha1.AppRelease) field.ErrorList {
	errs := validateRequiredLabels(newRelease)
	if oldRelease != nil {
		errs = append(errs, validateTenantImmutability(oldRelease, newRelease)...)
	}
	path := field.NewPath("spec", "appRef")
	if strings.TrimSpace(newRelease.Spec.AppRef) == "" {
		errs = append(errs, field.Required(path, "appRef is required"))
		return errs
	}
	var app appv1alpha1.App
	key := types.NamespacedName{Namespace: newRelease.Namespace, Name: newRelease.Spec.AppRef}
	if err := w.client.Get(ctx, key, &app); err != nil {
		if apierrors.IsNotFound(err) {
			errs = append(errs, field.Invalid(path, newRelease.Spec.AppRef, "referenced App does not exist"))
			return errs
		}
		w.logger.Errorw("Failed to fetch App for AppRelease", "name", key, "error", err)
		errs = append(errs, field.InternalError(path, fmt.Errorf("unable to verify App %q", newRelease.Spec.AppRef)))
		return errs
	}
	releaseLabels := newRelease.GetLabels()
	appLabels := app.GetLabels()
	if tenant := releaseLabels[labelTenant]; tenant != "" && appLabels[labelTenant] != tenant {
		errs = append(errs, field.Forbidden(path, fmt.Sprintf("App %q belongs to tenant %q", newRelease.Spec.AppRef, appLabels[labelTenant])))
	}
	if project := releaseLabels[labelProject]; project != "" && appLabels[labelProject] != project {
		errs = append(errs, field.Forbidden(path, fmt.Sprintf("App %q belongs to project %q", newRelease.Spec.AppRef, appLabels[labelProject])))
	}
	return errs
}

func (w *ServiceBindingWebhook) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&appv1alpha1.ServiceBinding{}).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

func (w *ServiceBindingWebhook) Default(_ context.Context, obj runtime.Object) error {
	binding, ok := obj.(*appv1alpha1.ServiceBinding)
	if !ok {
		return fmt.Errorf("expected ServiceBinding, got %T", obj)
	}
	defaultMetadata(binding)
	return nil
}

func (w *ServiceBindingWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (webhook.Warnings, error) {
	binding, ok := obj.(*appv1alpha1.ServiceBinding)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected ServiceBinding, got %T", obj))
	}
	if errs := w.validateBinding(ctx, nil, binding); len(errs) > 0 {
		return nil, apierrors.NewInvalid(appv1alpha1.GroupVersion.WithKind("ServiceBinding").GroupKind(), binding.Name, errs)
	}
	return nil, nil
}

func (w *ServiceBindingWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (webhook.Warnings, error) {
	newBinding, ok := newObj.(*appv1alpha1.ServiceBinding)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected ServiceBinding, got %T", newObj))
	}
	oldBinding, ok := oldObj.(*appv1alpha1.ServiceBinding)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected ServiceBinding, got %T", oldObj))
	}
	if errs := w.validateBinding(ctx, oldBinding, newBinding); len(errs) > 0 {
		return nil, apierrors.NewInvalid(appv1alpha1.GroupVersion.WithKind("ServiceBinding").GroupKind(), newBinding.Name, errs)
	}
	return nil, nil
}

func (w *ServiceBindingWebhook) ValidateDelete(context.Context, runtime.Object) (webhook.Warnings, error) {
	return nil, nil
}

func (w *ServiceBindingWebhook) validateBinding(ctx context.Context, oldBinding, newBinding *appv1alpha1.ServiceBinding) field.ErrorList {
	errs := validateRequiredLabels(newBinding)
	if oldBinding != nil {
		errs = append(errs, validateTenantImmutability(oldBinding, newBinding)...)
	}
	tenant := strings.TrimSpace(newBinding.GetLabels()[labelTenant])
	if tenant == "" {
		return errs
	}

	errs = append(errs, w.validateBindingConsumer(ctx, newBinding, tenant)...)
	errs = append(errs, w.validateBindingProvider(ctx, newBinding, tenant)...)
	return errs
}

func (w *ServiceBindingWebhook) validateBindingConsumer(ctx context.Context, binding *appv1alpha1.ServiceBinding, tenant string) field.ErrorList {
	var errs field.ErrorList
	path := field.NewPath("spec", "consumerRef")
	switch binding.Spec.Consumer.Type {
	case appv1alpha1.ServiceBindingConsumerTypeApp:
		var app appv1alpha1.App
		key := types.NamespacedName{Namespace: binding.Namespace, Name: binding.Spec.Consumer.Name}
		if err := w.client.Get(ctx, key, &app); err != nil {
			if apierrors.IsNotFound(err) {
				errs = append(errs, field.Invalid(path.Child("name"), binding.Spec.Consumer.Name, "referenced App does not exist"))
				return errs
			}
			w.logger.Errorw("Failed to fetch consumer App", "name", key, "error", err)
			errs = append(errs, field.InternalError(path.Child("name"), fmt.Errorf("unable to verify App %q", binding.Spec.Consumer.Name)))
			return errs
		}
		if appTenant := app.GetLabels()[labelTenant]; appTenant != tenant {
			errs = append(errs, field.Forbidden(path.Child("name"), fmt.Sprintf("App %q belongs to tenant %q", binding.Spec.Consumer.Name, appTenant)))
		}
	case appv1alpha1.ServiceBindingConsumerTypeServiceAccount:
		var sa corev1.ServiceAccount
		key := types.NamespacedName{Namespace: binding.Namespace, Name: binding.Spec.Consumer.Name}
		if err := w.client.Get(ctx, key, &sa); err != nil {
			if apierrors.IsNotFound(err) {
				errs = append(errs, field.Invalid(path.Child("name"), binding.Spec.Consumer.Name, "serviceAccount does not exist"))
				return errs
			}
			w.logger.Errorw("Failed to fetch consumer ServiceAccount", "name", key, "error", err)
			errs = append(errs, field.InternalError(path.Child("name"), fmt.Errorf("unable to verify ServiceAccount %q", binding.Spec.Consumer.Name)))
			return errs
		}
		if sa.Namespace != binding.Namespace {
			errs = append(errs, field.Forbidden(path.Child("name"), fmt.Sprintf("serviceAccount %q must reside in namespace %q", binding.Spec.Consumer.Name, binding.Namespace)))
		}
	default:
		errs = append(errs, field.Invalid(path.Child("type"), binding.Spec.Consumer.Type, "unsupported consumer type"))
	}
	return errs
}

func (w *ServiceBindingWebhook) validateBindingProvider(ctx context.Context, binding *appv1alpha1.ServiceBinding, tenant string) field.ErrorList {
	var errs field.ErrorList
	path := field.NewPath("spec", "providerRef")
	if strings.TrimSpace(binding.Spec.Provider.Name) == "" {
		errs = append(errs, field.Required(path.Child("name"), "provider name is required"))
		return errs
	}
	key := types.NamespacedName{Namespace: binding.Namespace, Name: binding.Spec.Provider.Name}
	switch binding.Spec.Provider.Type {
	case appv1alpha1.ServiceBindingProviderTypeDatabase:
		var db appv1alpha1.DatabaseInstance
		if err := w.client.Get(ctx, key, &db); err != nil {
			errs = append(errs, providerLookupError(path, binding, err))
			return errs
		}
		if db.GetLabels()[labelTenant] != tenant {
			errs = append(errs, field.Forbidden(path.Child("name"), fmt.Sprintf("DatabaseInstance %q belongs to tenant %q", binding.Spec.Provider.Name, db.GetLabels()[labelTenant])))
		}
	case appv1alpha1.ServiceBindingProviderTypeCache:
		var cache appv1alpha1.CacheInstance
		if err := w.client.Get(ctx, key, &cache); err != nil {
			errs = append(errs, providerLookupError(path, binding, err))
			return errs
		}
		if cache.GetLabels()[labelTenant] != tenant {
			errs = append(errs, field.Forbidden(path.Child("name"), fmt.Sprintf("CacheInstance %q belongs to tenant %q", binding.Spec.Provider.Name, cache.GetLabels()[labelTenant])))
		}
	case appv1alpha1.ServiceBindingProviderTypeQueue:
		var queue appv1alpha1.QueueInstance
		if err := w.client.Get(ctx, key, &queue); err != nil {
			errs = append(errs, providerLookupError(path, binding, err))
			return errs
		}
		if queue.GetLabels()[labelTenant] != tenant {
			errs = append(errs, field.Forbidden(path.Child("name"), fmt.Sprintf("QueueInstance %q belongs to tenant %q", binding.Spec.Provider.Name, queue.GetLabels()[labelTenant])))
		}
	default:
		errs = append(errs, field.Invalid(path.Child("type"), binding.Spec.Provider.Type, "unsupported provider type"))
	}
	return errs
}

func providerLookupError(path *field.Path, binding *appv1alpha1.ServiceBinding, err error) *field.Error {
	if apierrors.IsNotFound(err) {
		return field.Invalid(path.Child("name"), binding.Spec.Provider.Name, "referenced provider does not exist")
	}
	return field.InternalError(path.Child("name"), fmt.Errorf("unable to verify provider %q: %v", binding.Spec.Provider.Name, err))
}

func (w *AppWebhook) projectForApp(ctx context.Context, app *appv1alpha1.App, tenant string) (*appv1alpha1.Project, field.ErrorList) {
	var errs field.ErrorList
	projectName := strings.TrimSpace(app.GetLabels()[labelProject])
	if projectName == "" {
		return nil, errs
	}
	var project appv1alpha1.Project
	key := types.NamespacedName{Namespace: app.Namespace, Name: projectName}
	if err := w.client.Get(ctx, key, &project); err != nil {
		labelPath := field.NewPath("metadata", "labels").Key(labelProject)
		if apierrors.IsNotFound(err) {
			errs = append(errs, field.Invalid(labelPath, projectName, "project label must reference an existing Project"))
			return nil, errs
		}
		w.logger.Errorw("Failed to fetch Project for App", "name", key, "error", err)
		errs = append(errs, field.InternalError(labelPath, fmt.Errorf("unable to verify Project %q", projectName)))
		return nil, errs
	}
	if tenant != "" && project.Spec.TenantRef != "" && project.Spec.TenantRef != tenant {
		errs = append(errs, field.Forbidden(field.NewPath("metadata", "labels").Key(labelTenant), fmt.Sprintf("Project %q belongs to tenant %q", projectName, project.Spec.TenantRef)))
	}
	if projectTenant := strings.TrimSpace(project.GetLabels()[labelTenant]); tenant != "" && projectTenant != "" && projectTenant != tenant {
		errs = append(errs, field.Forbidden(field.NewPath("metadata", "labels").Key(labelTenant), fmt.Sprintf("Project %q labels tenant %q", projectName, projectTenant)))
	}
	return &project, errs
}

func (w *AppWebhook) validateServicePolicy(ctx context.Context, app *appv1alpha1.App, project *appv1alpha1.Project) field.ErrorList {
	var errs field.ErrorList
	if app.Spec.ServiceProfile == nil {
		return errs
	}
	profile := app.Spec.ServiceProfile
	path := field.NewPath("spec", "serviceProfile")
	requestedType := corev1.ServiceTypeClusterIP
	if profile.Type != "" {
		requestedType = profile.Type
	}

	// No project context -> default to forbidding privileged exposure.
	if project == nil || strings.TrimSpace(project.Spec.NetworkPolicyProfileRef) == "" {
		if requestedType == corev1.ServiceTypeLoadBalancer {
			errs = append(errs, field.Forbidden(path.Child("type"), "LoadBalancer services require an approved service policy"))
		}
		if len(profile.ExternalIPs) > 0 {
			errs = append(errs, field.Forbidden(path.Child("externalIPs"), "External IPs require an approved service policy"))
		}
		return errs
	}

	var networkProfile appv1alpha1.NetworkPolicyProfile
	profileKey := types.NamespacedName{Name: project.Spec.NetworkPolicyProfileRef}
	if err := w.client.Get(ctx, profileKey, &networkProfile); err != nil {
		if apierrors.IsNotFound(err) {
			errs = append(errs, field.Invalid(path, project.Spec.NetworkPolicyProfileRef, "referenced NetworkPolicyProfile does not exist"))
			return errs
		}
		w.logger.Errorw("Failed to fetch NetworkPolicyProfile", "name", profileKey, "error", err)
		errs = append(errs, field.InternalError(path, fmt.Errorf("unable to verify NetworkPolicyProfile %q", project.Spec.NetworkPolicyProfileRef)))
		return errs
	}

	policy := networkProfile.Spec.ServicePolicy
	allowedTypes := sets.NewString()
	if policy != nil {
		for _, t := range policy.AllowedTypes {
			allowedTypes.Insert(string(t))
		}
	}
	typeAllowed := requestedType == corev1.ServiceTypeClusterIP
	if requestedType != corev1.ServiceTypeClusterIP {
		if policy == nil || allowedTypes.Len() == 0 {
			typeAllowed = false
		} else {
			typeAllowed = allowedTypes.Has(string(requestedType))
		}
	}
	if !typeAllowed {
		errs = append(errs, field.Forbidden(path.Child("type"), fmt.Sprintf("service type %q is not permitted by profile %q", requestedType, networkProfile.Name)))
	}

	if len(profile.ExternalIPs) > 0 {
		if policy == nil || len(policy.AllowedExternalIPs) == 0 {
			errs = append(errs, field.Forbidden(path.Child("externalIPs"), "external IPs are not allowed by the service policy"))
		} else {
			allowedIPs := sets.NewString(policy.AllowedExternalIPs...)
			for idx, ip := range profile.ExternalIPs {
				if !allowedIPs.Has(ip) {
					errs = append(errs, field.Forbidden(path.Child("externalIPs").Index(idx), fmt.Sprintf("external IP %q is not in the allowlist", ip)))
				}
			}
		}
	}

	return errs
}

func (w *BucketWebhook) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&appv1alpha1.Bucket{}).
		WithValidator(w).
		WithDefaulter(w).
		Complete()
}

func (w *BucketWebhook) Default(_ context.Context, obj runtime.Object) error {
	bucket, ok := obj.(*appv1alpha1.Bucket)
	if !ok {
		return fmt.Errorf("expected Bucket, got %T", obj)
	}
	defaultMetadata(bucket)
	return nil
}

func (w *BucketWebhook) ValidateCreate(ctx context.Context, obj runtime.Object) (webhook.Warnings, error) {
	bucket, ok := obj.(*appv1alpha1.Bucket)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected Bucket, got %T", obj))
	}
	if errs := w.validateBucket(ctx, nil, bucket); len(errs) > 0 {
		return nil, apierrors.NewInvalid(appv1alpha1.GroupVersion.WithKind("Bucket").GroupKind(), bucket.Name, errs)
	}
	return nil, nil
}

func (w *BucketWebhook) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (webhook.Warnings, error) {
	newBucket, ok := newObj.(*appv1alpha1.Bucket)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected Bucket, got %T", newObj))
	}
	oldBucket, ok := oldObj.(*appv1alpha1.Bucket)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected Bucket, got %T", oldObj))
	}
	if errs := w.validateBucket(ctx, oldBucket, newBucket); len(errs) > 0 {
		return nil, apierrors.NewInvalid(appv1alpha1.GroupVersion.WithKind("Bucket").GroupKind(), newBucket.Name, errs)
	}
	return nil, nil
}

func (w *BucketWebhook) ValidateDelete(context.Context, runtime.Object) (webhook.Warnings, error) {
	return nil, nil
}

func (w *BucketWebhook) validateBucket(ctx context.Context, oldBucket, newBucket *appv1alpha1.Bucket) field.ErrorList {
	errs := validateRequiredLabels(newBucket)
	if oldBucket != nil {
		errs = append(errs, validateTenantImmutability(oldBucket, newBucket)...)
	}
	tenant := strings.TrimSpace(newBucket.GetLabels()[labelTenant])
	project := strings.TrimSpace(newBucket.GetLabels()[labelProject])
	app := strings.TrimSpace(newBucket.GetLabels()[labelApp])
	if len(newBucket.Spec.PolicyRefs) == 0 {
		return errs
	}
	path := field.NewPath("spec", "policyRefs")
	for i, name := range newBucket.Spec.PolicyRefs {
		var policy appv1alpha1.BucketPolicy
		key := types.NamespacedName{Namespace: newBucket.Namespace, Name: name}
		if err := w.client.Get(ctx, key, &policy); err != nil {
			if apierrors.IsNotFound(err) {
				errs = append(errs, field.Invalid(path.Index(i), name, "referenced BucketPolicy does not exist"))
				continue
			}
			w.logger.Errorw("Failed to fetch BucketPolicy", "name", key, "error", err)
			errs = append(errs, field.InternalError(path.Index(i), fmt.Errorf("unable to verify BucketPolicy %q", name)))
			continue
		}
		policyLabels := policy.GetLabels()
		if tenant != "" && policyLabels[labelTenant] != tenant {
			errs = append(errs, field.Forbidden(path.Index(i), fmt.Sprintf("BucketPolicy %q belongs to tenant %q", name, policyLabels[labelTenant])))
		}
		if project != "" && policyLabels[labelProject] != project {
			errs = append(errs, field.Forbidden(path.Index(i), fmt.Sprintf("BucketPolicy %q belongs to project %q", name, policyLabels[labelProject])))
		}
		if app != "" && policyLabels[labelApp] != app {
			errs = append(errs, field.Forbidden(path.Index(i), fmt.Sprintf("BucketPolicy %q belongs to app %q", name, policyLabels[labelApp])))
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
	defaultMetadata(job)
	return nil
}

// ValidateCreate enforces pod security restrictions for tenant Jobs.
func (w *JobWebhook) ValidateCreate(_ context.Context, obj runtime.Object) (webhook.Warnings, error) {
	job, ok := obj.(*appv1alpha1.Job)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected Job, got %T", obj))
	}
	if errs := w.validateJob(nil, job); len(errs) > 0 {
		return nil, apierrors.NewInvalid(appv1alpha1.GroupVersion.WithKind("Job").GroupKind(), job.Name, errs)
	}
	return nil, nil
}

// ValidateUpdate reuses create-time enforcement for Job updates.
func (w *JobWebhook) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) (webhook.Warnings, error) {
	job, ok := newObj.(*appv1alpha1.Job)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected Job, got %T", newObj))
	}
	var oldJob *appv1alpha1.Job
	if oldObj != nil {
		var okOld bool
		oldJob, okOld = oldObj.(*appv1alpha1.Job)
		if !okOld {
			return nil, apierrors.NewBadRequest(fmt.Sprintf("expected Job, got %T", oldObj))
		}
	}
	if errs := w.validateJob(oldJob, job); len(errs) > 0 {
		return nil, apierrors.NewInvalid(appv1alpha1.GroupVersion.WithKind("Job").GroupKind(), job.Name, errs)
	}
	return nil, nil
}

// ValidateDelete performs no-op validation for Job deletions.
func (w *JobWebhook) ValidateDelete(context.Context, runtime.Object) (webhook.Warnings, error) {
	return nil, nil
}

func (w *JobWebhook) validateJob(oldJob, job *appv1alpha1.Job) field.ErrorList {
	errs := validateRequiredLabels(job)
	if oldJob != nil {
		errs = append(errs, validateTenantImmutability(oldJob, job)...)
	}
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
