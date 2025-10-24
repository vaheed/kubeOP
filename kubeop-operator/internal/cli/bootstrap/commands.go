package bootstrap

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	appv1alpha1 "github.com/vaheed/kubeOP/kubeop-operator/apis/paas/v1alpha1"
	bootstrapcore "github.com/vaheed/kubeOP/kubeop-operator/internal/bootstrap"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func newInitCommand(streams IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Install CRDs, RBAC, and webhook configurations",
		RunE: func(cmd *cobra.Command, args []string) error {
			state := getState(cmd)
			if err := requireConfirmation(state); err != nil {
				return err
			}
			ctx := cmd.Context()
			if err := bootstrapcore.EnsureCRDs(ctx, state.config, state.logger); err != nil {
				return err
			}
			summaries := make([]ApplySummary, 0)
			applied := make([]client.Object, 0)
			applyGroup := func(desc string, objs []client.Object) error {
				for _, obj := range objs {
					summary, err := state.runner.ApplyObject(ctx, obj)
					if err != nil {
						return fmt.Errorf("apply %s: %w", desc, err)
					}
					summaries = append(summaries, summary)
					applied = append(applied, obj)
				}
				return nil
			}
			rbac, err := LoadRBACManifests()
			if err != nil {
				return err
			}
			if err := applyGroup("rbac", rbac); err != nil {
				return err
			}
			webhooks, err := LoadWebhookManifests()
			if err != nil {
				return err
			}
			if err := applyGroup("webhooks", webhooks); err != nil {
				return err
			}
			table := SummariesToTable(summaries)
			return RenderOutput(streams.Out, state.output, state.runner.EncodeYAML, table, applied)
		},
	}
	return cmd
}

func newDefaultsCommand(streams IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "defaults",
		Short: "Apply default profiles (plans, network policies, runtime classes)",
		RunE: func(cmd *cobra.Command, args []string) error {
			state := getState(cmd)
			if err := requireConfirmation(state); err != nil {
				return err
			}
			ctx := cmd.Context()
			defaults, err := LoadDefaultManifests()
			if err != nil {
				return err
			}
			summaries := make([]ApplySummary, 0, len(defaults))
			applied := make([]client.Object, 0, len(defaults))
			for _, obj := range defaults {
				summary, err := state.runner.ApplyObject(ctx, obj)
				if err != nil {
					return err
				}
				summaries = append(summaries, summary)
				applied = append(applied, obj)
			}
			table := SummariesToTable(summaries)
			return RenderOutput(streams.Out, state.output, state.runner.EncodeYAML, table, applied)
		},
	}
	return cmd
}

func newTenantCommand(streams IOStreams) *cobra.Command {
	tenantCmd := &cobra.Command{
		Use:   "tenant",
		Short: "Manage tenant resources",
	}
	var input TenantInput
	create := &cobra.Command{
		Use:   "create",
		Short: "Create or update a tenant",
		RunE: func(cmd *cobra.Command, args []string) error {
			state := getState(cmd)
			if err := requireConfirmation(state); err != nil {
				return err
			}
			tenant, err := BuildTenant(input)
			if err != nil {
				return err
			}
			summary, err := state.runner.ApplyObject(cmd.Context(), tenant)
			if err != nil {
				return err
			}
			table := [][]string{
				{"TENANT", "DISPLAY NAME", "BILLING ACCOUNT"},
				{summary.Name, tenant.Spec.DisplayName, tenant.Spec.BillingAccountRef},
			}
			return RenderOutput(streams.Out, state.output, state.runner.EncodeYAML, table, []client.Object{tenant})
		},
	}
	create.Flags().StringVar(&input.Name, "name", "", "Tenant resource name")
	create.Flags().StringVar(&input.DisplayName, "display-name", "", "Tenant display name")
	create.Flags().StringVar(&input.BillingAccountRef, "billing-account", "", "Billing account reference")
	_ = create.MarkFlagRequired("name")
	_ = create.MarkFlagRequired("billing-account")
	tenantCmd.AddCommand(create)
	return tenantCmd
}

func newProjectCommand(streams IOStreams) *cobra.Command {
	projectCmd := &cobra.Command{
		Use:   "project",
		Short: "Manage project resources",
	}
	var (
		input ProjectInput
		env   string
	)
	create := &cobra.Command{
		Use:   "create",
		Short: "Create or update a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			state := getState(cmd)
			if err := requireConfirmation(state); err != nil {
				return err
			}
			input.Environment = appv1alpha1.ProjectEnvironment(env)
			project, err := BuildProject(input)
			if err != nil {
				return err
			}
			summary, err := state.runner.ApplyObject(cmd.Context(), project)
			if err != nil {
				return err
			}
			table := [][]string{
				{"PROJECT", "TENANT", "NAMESPACE", "ENV", "PURPOSE"},
				{summary.Name, project.Spec.TenantRef, project.Spec.NamespaceName, string(project.Spec.Environment), project.Spec.Purpose},
			}
			return RenderOutput(streams.Out, state.output, state.runner.EncodeYAML, table, []client.Object{project})
		},
	}
	create.Flags().StringVar(&input.Name, "name", "", "Project name")
	create.Flags().StringVar(&input.Namespace, "namespace", "", "Kubernetes namespace for the project")
	create.Flags().StringVar(&input.TenantRef, "tenant", "", "Tenant reference")
	create.Flags().StringVar(&input.Purpose, "purpose", "", "Project purpose")
	create.Flags().StringVar(&env, "environment", string(appv1alpha1.ProjectEnvironmentDev), "Project environment (dev|stage|prod)")
	_ = create.MarkFlagRequired("name")
	_ = create.MarkFlagRequired("namespace")
	_ = create.MarkFlagRequired("tenant")
	_ = create.MarkFlagRequired("purpose")
	projectCmd.AddCommand(create)
	return projectCmd
}

func newDomainCommand(streams IOStreams) *cobra.Command {
	domainCmd := &cobra.Command{
		Use:   "domain",
		Short: "Manage domain attachments",
	}
	var input DomainInput
	create := &cobra.Command{
		Use:   "attach",
		Short: "Attach a domain to a tenant",
		RunE: func(cmd *cobra.Command, args []string) error {
			state := getState(cmd)
			if err := requireConfirmation(state); err != nil {
				return err
			}
			domain, err := BuildDomain(input)
			if err != nil {
				return err
			}
			summary, err := state.runner.ApplyObject(cmd.Context(), domain)
			if err != nil {
				return err
			}
			table := [][]string{
				{"DOMAIN", "FQDN", "TENANT", "DNS PROVIDER", "CERT POLICY"},
				{summary.Name, domain.Spec.FQDN, domain.Spec.TenantRef, domain.Spec.DNSProviderRef, domain.Spec.CertificatePolicyRef},
			}
			return RenderOutput(streams.Out, state.output, state.runner.EncodeYAML, table, []client.Object{domain})
		},
	}
	create.Flags().StringVar(&input.Name, "name", "", "Domain resource name")
	create.Flags().StringVar(&input.FQDN, "fqdn", "", "Fully qualified domain name")
	create.Flags().StringVar(&input.TenantRef, "tenant", "", "Tenant reference")
	create.Flags().StringVar(&input.DNSProviderRef, "dns-provider", "", "DNS provider reference")
	create.Flags().StringVar(&input.CertificatePolicyRef, "certificate-policy", "", "Certificate policy reference")
	_ = create.MarkFlagRequired("name")
	_ = create.MarkFlagRequired("fqdn")
	_ = create.MarkFlagRequired("tenant")
	_ = create.MarkFlagRequired("dns-provider")
	domainCmd.AddCommand(create)
	return domainCmd
}

func newRegistryCommand(streams IOStreams) *cobra.Command {
	registryCmd := &cobra.Command{
		Use:   "registry",
		Short: "Manage registry credentials",
	}
	var (
		input    RegistryInput
		typeFlag string
	)
	create := &cobra.Command{
		Use:   "add",
		Short: "Register a container registry credential",
		RunE: func(cmd *cobra.Command, args []string) error {
			state := getState(cmd)
			if err := requireConfirmation(state); err != nil {
				return err
			}
			parsedType, err := parseRegistryType(typeFlag)
			if err != nil {
				return err
			}
			input.Type = parsedType
			credential, err := BuildRegistryCredential(input)
			if err != nil {
				return err
			}
			summary, err := state.runner.ApplyObject(cmd.Context(), credential)
			if err != nil {
				return err
			}
			table := [][]string{
				{"REGISTRY", "TYPE", "TENANT", "SECRET"},
				{summary.Name, string(credential.Spec.Type), credential.Spec.TenantRef, credential.Spec.SecretRef},
			}
			return RenderOutput(streams.Out, state.output, state.runner.EncodeYAML, table, []client.Object{credential})
		},
	}
	create.Flags().StringVar(&input.Name, "name", "", "Registry credential name")
	create.Flags().StringVar(&input.TenantRef, "tenant", "", "Tenant reference")
	create.Flags().StringVar(&input.SecretRef, "secret", "", "Secret reference containing registry credentials")
	create.Flags().StringVar(&typeFlag, "type", string(appv1alpha1.RegistryCredentialDockerHub), "Registry type (dockerHub|ecr|gcr|harbor)")
	_ = create.MarkFlagRequired("name")
	_ = create.MarkFlagRequired("tenant")
	_ = create.MarkFlagRequired("secret")
	registryCmd.AddCommand(create)
	return registryCmd
}

func parseRegistryType(value string) (appv1alpha1.RegistryCredentialType, error) {
	switch strings.ToLower(value) {
	case strings.ToLower(string(appv1alpha1.RegistryCredentialDockerHub)):
		return appv1alpha1.RegistryCredentialDockerHub, nil
	case strings.ToLower(string(appv1alpha1.RegistryCredentialECR)):
		return appv1alpha1.RegistryCredentialECR, nil
	case strings.ToLower(string(appv1alpha1.RegistryCredentialGCR)):
		return appv1alpha1.RegistryCredentialGCR, nil
	case strings.ToLower(string(appv1alpha1.RegistryCredentialHarbor)):
		return appv1alpha1.RegistryCredentialHarbor, nil
	default:
		return "", fmt.Errorf("unsupported registry type %q", value)
	}
}
