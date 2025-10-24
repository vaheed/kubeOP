package webhooks

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/vaheed/kubeOP/kubeop-operator/apis/paas/v1alpha1"
	"go.uber.org/zap"
)

func newTestAppWebhook(t *testing.T, objects ...runtime.Object) *AppWebhook {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add app scheme: %v", err)
	}
	cli := fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build()
	return &AppWebhook{client: cli, logger: zap.NewNop().Sugar()}
}

func TestAppWebhookRejectsMissingTenantLabel(t *testing.T) {
	webhook := newTestAppWebhook(t)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "team-a",
			Labels: map[string]string{
				labelProject: "proj",
				labelApp:     "app",
			},
		},
	}
	_, err := webhook.ValidateCreate(context.Background(), app)
	if err == nil {
		t.Fatalf("expected error for missing tenant label")
	}
	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected invalid error, got %v", err)
	}
}

func TestAppWebhookRejectsCrossTenantSecretRef(t *testing.T) {
	secret := &appv1alpha1.SecretRef{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "shared",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-b",
				labelProject: "proj",
				labelApp:     "app",
			},
		},
	}
	webhook := newTestAppWebhook(t, secret)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "demo",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-a",
				labelProject: "proj",
				labelApp:     "app",
			},
		},
		Spec: appv1alpha1.AppSpec{
			Type:        appv1alpha1.AppTypeRaw,
			SecretsRefs: []string{"shared"},
		},
	}
	_, err := webhook.ValidateCreate(context.Background(), app)
	if err == nil {
		t.Fatalf("expected cross-tenant validation error")
	}
	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected invalid error, got %v", err)
	}
}

func TestJobWebhookRejectsHostNetwork(t *testing.T) {
	job := &appv1alpha1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "batch",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-a",
				labelProject: "proj",
				labelApp:     "app",
			},
		},
		Spec: appv1alpha1.JobSpec{
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{HostNetwork: true}},
		},
	}
	webhook := &JobWebhook{logger: zap.NewNop().Sugar()}
	_, err := webhook.ValidateCreate(context.Background(), job)
	if err == nil {
		t.Fatalf("expected hostNetwork to be rejected")
	}
	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected invalid error, got %v", err)
	}
}

func TestJobWebhookRequiresRunAsRootJustification(t *testing.T) {
	runAsUser := int64(0)
	job := &appv1alpha1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "root-job",
			Namespace: "team-a",
			Labels: map[string]string{
				labelTenant:  "tenant-a",
				labelProject: "proj",
				labelApp:     "app",
			},
		},
		Spec: appv1alpha1.JobSpec{
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{
				Name:  "runner",
				Image: "busybox",
				SecurityContext: &corev1.SecurityContext{
					RunAsUser: &runAsUser,
				},
			}}}},
		},
	}
	webhook := &JobWebhook{logger: zap.NewNop().Sugar()}
	_, err := webhook.ValidateCreate(context.Background(), job)
	if err == nil {
		t.Fatalf("expected runAsRoot to require justification")
	}
	if !apierrors.IsInvalid(err) {
		t.Fatalf("expected invalid error, got %v", err)
	}

	job = job.DeepCopy()
	job.Annotations = map[string]string{annotationRunAsRootJustification: "needs root for init"}
	if _, err := webhook.ValidateCreate(context.Background(), job); err != nil {
		t.Fatalf("expected justification to allow runAsRoot, got %v", err)
	}
}
