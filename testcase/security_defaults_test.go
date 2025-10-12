package testcase

import (
	corev1 "k8s.io/api/core/v1"
	"kubeop/internal/service"
	"testing"
)

func TestDefaultContainerSecurityContextRestricted(t *testing.T) {
	sc := service.DefaultContainerSecurityContextRestricted()
	if sc == nil {
		t.Fatalf("security context is nil")
	}
	if sc.RunAsNonRoot == nil || !*sc.RunAsNonRoot {
		t.Fatalf("RunAsNonRoot must be true")
	}
	if sc.AllowPrivilegeEscalation == nil || *sc.AllowPrivilegeEscalation {
		t.Fatalf("AllowPrivilegeEscalation must be false")
	}
	if sc.ReadOnlyRootFilesystem == nil || !*sc.ReadOnlyRootFilesystem {
		t.Fatalf("ReadOnlyRootFilesystem must be true")
	}
	if sc.Capabilities == nil || len(sc.Capabilities.Drop) == 0 {
		t.Fatalf("Capabilities.Drop must include ALL")
	}
	hasAll := false
	for _, c := range sc.Capabilities.Drop {
		if c == corev1.Capability("ALL") {
			hasAll = true
			break
		}
	}
	if !hasAll {
		t.Fatalf("Capabilities.Drop must contain ALL")
	}
	if sc.SeccompProfile == nil || sc.SeccompProfile.Type != corev1.SeccompProfileTypeRuntimeDefault {
		t.Fatalf("SeccompProfile must be runtime/default")
	}
}
