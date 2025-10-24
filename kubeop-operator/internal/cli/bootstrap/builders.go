package bootstrap

import (
	"context"
	"fmt"

	appv1alpha1 "github.com/vaheed/kubeOP/kubeop-operator/apis/paas/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

type TenantInput struct {
	Name              string
	DisplayName       string
	BillingAccountRef string
}

type ProjectInput struct {
	Name        string
	Namespace   string
	TenantRef   string
	Purpose     string
	Environment appv1alpha1.ProjectEnvironment
}

type DomainInput struct {
	Name                 string
	FQDN                 string
	TenantRef            string
	DNSProviderRef       string
	CertificatePolicyRef string
}

type RegistryInput struct {
	Name      string
	TenantRef string
	Type      appv1alpha1.RegistryCredentialType
	SecretRef string
}

func BuildTenant(input TenantInput) (*appv1alpha1.Tenant, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("tenant name is required")
	}
	if errs := validation.IsDNS1123Label(input.Name); len(errs) > 0 {
		return nil, fmt.Errorf("tenant name invalid: %v", errs)
	}
	if input.BillingAccountRef == "" {
		return nil, fmt.Errorf("billing account reference is required")
	}
	if input.DisplayName == "" {
		input.DisplayName = input.Name
	}
	tenant := &appv1alpha1.Tenant{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appv1alpha1.GroupVersion.String(),
			Kind:       "Tenant",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: input.Name,
			Labels: map[string]string{
				"paas.kubeop.io/tenant": input.Name,
			},
		},
		Spec: appv1alpha1.TenantSpec{
			DisplayName:       input.DisplayName,
			BillingAccountRef: input.BillingAccountRef,
		},
	}
	return tenant, nil
}

func BuildProject(input ProjectInput) (*appv1alpha1.Project, error) {
	if input.Name == "" || input.Namespace == "" || input.TenantRef == "" {
		return nil, fmt.Errorf("project name, namespace, and tenant are required")
	}
	if input.Environment == "" {
		input.Environment = appv1alpha1.ProjectEnvironmentDev
	}
	proj := &appv1alpha1.Project{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appv1alpha1.GroupVersion.String(),
			Kind:       "Project",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      input.Name,
			Namespace: input.Namespace,
			Labels: map[string]string{
				"paas.kubeop.io/tenant":  input.TenantRef,
				"paas.kubeop.io/project": input.Name,
				"paas.kubeop.io/env":     string(input.Environment),
			},
		},
		Spec: appv1alpha1.ProjectSpec{
			TenantRef:     input.TenantRef,
			Purpose:       input.Purpose,
			Environment:   input.Environment,
			NamespaceName: input.Namespace,
			PSAPreset:     appv1alpha1.SecurityPresetBaseline,
		},
	}
	return proj, nil
}

func BuildDomain(input DomainInput) (*appv1alpha1.Domain, error) {
	if input.Name == "" || input.FQDN == "" || input.TenantRef == "" || input.DNSProviderRef == "" {
		return nil, fmt.Errorf("domain name, fqdn, tenant, and dns provider are required")
	}
	domain := &appv1alpha1.Domain{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appv1alpha1.GroupVersion.String(),
			Kind:       "Domain",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: input.Name,
			Labels: map[string]string{
				"paas.kubeop.io/tenant": input.TenantRef,
			},
		},
		Spec: appv1alpha1.DomainSpec{
			FQDN:                 input.FQDN,
			TenantRef:            input.TenantRef,
			DNSProviderRef:       input.DNSProviderRef,
			CertificatePolicyRef: input.CertificatePolicyRef,
		},
	}
	return domain, nil
}

func BuildRegistryCredential(input RegistryInput) (*appv1alpha1.RegistryCredential, error) {
	if input.Name == "" || input.SecretRef == "" || input.TenantRef == "" {
		return nil, fmt.Errorf("registry name, tenant, and secret are required")
	}
	cred := &appv1alpha1.RegistryCredential{
		TypeMeta: metav1.TypeMeta{
			APIVersion: appv1alpha1.GroupVersion.String(),
			Kind:       "RegistryCredential",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: input.Name,
			Labels: map[string]string{
				"paas.kubeop.io/tenant": input.TenantRef,
			},
		},
		Spec: appv1alpha1.RegistryCredentialSpec{
			Type:      input.Type,
			SecretRef: input.SecretRef,
			TenantRef: input.TenantRef,
		},
	}
	return cred, nil
}

func ApplyObject(ctx context.Context, c client.Client, scheme *runtime.Scheme, obj client.Object, fieldOwner string) error {
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if c == nil {
		return fmt.Errorf("kubernetes client is required")
	}
	if scheme == nil {
		return fmt.Errorf("runtime scheme is required")
	}
	if obj == nil {
		return fmt.Errorf("object is required")
	}
	if fieldOwner == "" {
		fieldOwner = "kubeop-bootstrap"
	}
	gvk, err := apiutil.GVKForObject(obj, scheme)
	if err != nil {
		return fmt.Errorf("determine gvk: %w", err)
	}
	obj.GetObjectKind().SetGroupVersionKind(gvk)
	obj.SetManagedFields(nil)
	obj.SetResourceVersion("")
	return c.Patch(ctx, obj, client.Apply, client.FieldOwner(fieldOwner), client.ForceOwnership)
}
