// Package v1alpha1 contains API Schema definitions for the kubeop.io v1alpha1 API group.
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// GroupVersion identifies the API group and version for kubeOP custom resources.
	GroupVersion = schema.GroupVersion{Group: "kubeop.io", Version: "v1alpha1"}

	// SchemeBuilder accumulates functions that register the kubeOP operator types with a scheme.
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme applies all registered types to the provided scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&App{},
		&AppList{},
	)
	metav1.AddToGroupVersion(scheme, GroupVersion)
	return nil
}
