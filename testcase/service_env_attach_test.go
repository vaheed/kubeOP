package testcase

import (
	"testing"

	"kubeop/internal/service"

	corev1 "k8s.io/api/core/v1"
)

func TestAttachConfigMapEnvFrom(t *testing.T) {
	t.Parallel()

	ctn := &corev1.Container{}
	service.AttachConfigMapEnv(ctn, "app-config", nil, "")
	if len(ctn.EnvFrom) != 1 {
		t.Fatalf("expected 1 envFrom entry, got %d", len(ctn.EnvFrom))
	}
	service.AttachConfigMapEnv(ctn, "app-config", nil, "")
	if len(ctn.EnvFrom) != 1 {
		t.Fatalf("duplicate envFrom should not be added, got %d", len(ctn.EnvFrom))
	}
}

func TestAttachConfigMapEnvKeys(t *testing.T) {
	t.Parallel()

	ctn := &corev1.Container{Env: []corev1.EnvVar{{Name: "EXISTING", Value: "keep"}}}
	service.AttachConfigMapEnv(ctn, "cfg", []string{"FOO", "BAR", "FOO"}, "APP_")
	if len(ctn.Env) != 3 {
		t.Fatalf("expected 3 env vars after attach, got %d", len(ctn.Env))
	}
	var foundFoo, foundBar bool
	for _, ev := range ctn.Env {
		switch ev.Name {
		case "APP_FOO":
			if ev.ValueFrom == nil || ev.ValueFrom.ConfigMapKeyRef == nil || ev.ValueFrom.ConfigMapKeyRef.Key != "FOO" {
				t.Fatalf("APP_FOO not wired to configmap key: %#v", ev)
			}
			foundFoo = true
		case "APP_BAR":
			if ev.ValueFrom == nil || ev.ValueFrom.ConfigMapKeyRef == nil || ev.ValueFrom.ConfigMapKeyRef.Key != "BAR" {
				t.Fatalf("APP_BAR not wired to configmap key: %#v", ev)
			}
			foundBar = true
		}
	}
	if !foundFoo || !foundBar {
		t.Fatalf("expected both APP_FOO and APP_BAR to be attached: %#v", ctn.Env)
	}
}

func TestDetachConfigMapEnv(t *testing.T) {
	t.Parallel()

	ctn := &corev1.Container{
		EnvFrom: []corev1.EnvFromSource{{ConfigMapRef: &corev1.ConfigMapEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cfg"}}}},
		Env:     []corev1.EnvVar{{Name: "FOO", ValueFrom: &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "cfg"}, Key: "FOO"}}}, {Name: "OTHER", Value: "keep"}},
	}
	service.DetachConfigMapEnv(ctn, "cfg")
	if len(ctn.EnvFrom) != 0 {
		t.Fatalf("expected envFrom to be cleared, got %#v", ctn.EnvFrom)
	}
	if len(ctn.Env) != 1 || ctn.Env[0].Name != "OTHER" {
		t.Fatalf("expected only OTHER env var to remain, got %#v", ctn.Env)
	}
}

func TestAttachSecretEnv(t *testing.T) {
	t.Parallel()

	ctn := &corev1.Container{}
	service.AttachSecretEnv(ctn, "cred", nil, "")
	if len(ctn.EnvFrom) != 1 {
		t.Fatalf("expected secret envFrom to be added, got %d", len(ctn.EnvFrom))
	}
	service.AttachSecretEnv(ctn, "cred", []string{"TOKEN"}, "")
	if len(ctn.Env) != 1 {
		t.Fatalf("expected TOKEN env var, got %#v", ctn.Env)
	}
	if ctn.Env[0].ValueFrom == nil || ctn.Env[0].ValueFrom.SecretKeyRef == nil || ctn.Env[0].ValueFrom.SecretKeyRef.Key != "TOKEN" {
		t.Fatalf("TOKEN env var should reference secret key: %#v", ctn.Env[0])
	}
}

func TestDetachSecretEnv(t *testing.T) {
	t.Parallel()

	ctn := &corev1.Container{
		EnvFrom: []corev1.EnvFromSource{{SecretRef: &corev1.SecretEnvSource{LocalObjectReference: corev1.LocalObjectReference{Name: "cred"}}}},
		Env:     []corev1.EnvVar{{Name: "TOKEN", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "cred"}, Key: "TOKEN"}}}, {Name: "KEEP", Value: "1"}},
	}
	service.DetachSecretEnv(ctn, "cred")
	if len(ctn.EnvFrom) != 0 {
		t.Fatalf("expected secret envFrom cleared, got %#v", ctn.EnvFrom)
	}
	if len(ctn.Env) != 1 || ctn.Env[0].Name != "KEEP" {
		t.Fatalf("expected KEEP env var to remain, got %#v", ctn.Env)
	}
}
