package testcase

import (
	"testing"

	appv1alpha1 "github.com/vaheed/kubeOP/kubeop-operator/apis/paas/v1alpha1"
)

func TestServiceBindingEnumerations(t *testing.T) {
	t.Helper()

	cases := map[string]struct {
		actual interface{}
		want   string
	}{
		"consumer-app": {
			actual: appv1alpha1.ServiceBindingConsumerTypeApp,
			want:   "app",
		},
		"consumer-service-account": {
			actual: appv1alpha1.ServiceBindingConsumerTypeServiceAccount,
			want:   "serviceAccount",
		},
		"provider-database": {
			actual: appv1alpha1.ServiceBindingProviderTypeDatabase,
			want:   "database",
		},
		"provider-cache": {
			actual: appv1alpha1.ServiceBindingProviderTypeCache,
			want:   "cache",
		},
		"provider-queue": {
			actual: appv1alpha1.ServiceBindingProviderTypeQueue,
			want:   "queue",
		},
		"injection-env": {
			actual: appv1alpha1.ServiceBindingInjectionTypeEnv,
			want:   "env",
		},
		"injection-file": {
			actual: appv1alpha1.ServiceBindingInjectionTypeFile,
			want:   "file",
		},
		"injection-secret": {
			actual: appv1alpha1.ServiceBindingInjectionTypeSecret,
			want:   "secret",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := ""
			switch v := tc.actual.(type) {
			case appv1alpha1.ServiceBindingConsumerType:
				got = string(v)
			case appv1alpha1.ServiceBindingProviderType:
				got = string(v)
			case appv1alpha1.ServiceBindingInjectionType:
				got = string(v)
			default:
				t.Fatalf("unsupported type %T", v)
			}

			if got != tc.want {
				t.Fatalf("unexpected value: got %q, want %q", got, tc.want)
			}
		})
	}
}
