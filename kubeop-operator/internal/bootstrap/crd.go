package bootstrap

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/yaml"
)

const (
	appCRDReadyTimeout  = 30 * time.Second
	appCRDReadyInterval = 500 * time.Millisecond
)

var waitForCRDReadyFn = waitForCRDReady

// EnsureAppCRD installs or updates the App CRD before the controller manager starts.
func EnsureAppCRD(ctx context.Context, cfg *rest.Config, logger *zap.SugaredLogger) error {
	if cfg == nil {
		return fmt.Errorf("kubernetes config is required")
	}
	if logger == nil {
		return fmt.Errorf("logger is required")
	}

	client, err := apiextensionsclient.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("build apiextensions client: %w", err)
	}

	return ensureAppCRDWithClient(ctx, client, logger)
}

func ensureAppCRDWithClient(ctx context.Context, client apiextensionsclient.Interface, logger *zap.SugaredLogger) error {
	if client == nil {
		return fmt.Errorf("apiextensions client is required")
	}

	if logger == nil {
		return fmt.Errorf("logger is required")
	}

	crd, err := loadAppCRD()
	if err != nil {
		return err
	}

	logger.Infow("Ensuring App CRD is installed", "name", crd.Name)

	existing, err := client.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crd.Name, metav1.GetOptions{})
	switch {
	case errors.IsNotFound(err):
		logger.Infow("Installing App CRD", "name", crd.Name)
		if _, err := client.ApiextensionsV1().CustomResourceDefinitions().Create(ctx, crd, metav1.CreateOptions{}); err != nil {
			return fmt.Errorf("create App CRD: %w", err)
		}
	case err != nil:
		return fmt.Errorf("get App CRD: %w", err)
	default:
		merged := crd.DeepCopy()
		merged.ResourceVersion = existing.ResourceVersion
		merged.UID = existing.UID
		merged.CreationTimestamp = existing.CreationTimestamp
		merged.ManagedFields = existing.ManagedFields
		merged.Generation = existing.Generation
		merged.Status = existing.Status
		merged.Labels = mergeStringMap(existing.Labels, crd.Labels)
		merged.Annotations = mergeStringMap(existing.Annotations, crd.Annotations)

		if needsUpdate(existing, merged) {
			logger.Infow("Updating App CRD to match bundled manifest", "name", crd.Name)
			if _, err := client.ApiextensionsV1().CustomResourceDefinitions().Update(ctx, merged, metav1.UpdateOptions{}); err != nil {
				return fmt.Errorf("update App CRD: %w", err)
			}
		}
	}

	if err := waitForCRDReadyFn(ctx, client, crd.Name); err != nil {
		return fmt.Errorf("wait for App CRD readiness: %w", err)
	}

	logger.Infow("App CRD ready", "name", crd.Name)
	return nil
}

func loadAppCRD() (*apiextensionsv1.CustomResourceDefinition, error) {
	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(appCRDManifest, &crd); err != nil {
		return nil, fmt.Errorf("unmarshal App CRD manifest: %w", err)
	}
	return &crd, nil
}

func needsUpdate(existing, desired *apiextensionsv1.CustomResourceDefinition) bool {
	if existing == nil || desired == nil {
		return false
	}

	if !apiequality.Semantic.DeepEqual(existing.Spec, desired.Spec) {
		return true
	}

	if !mapContains(existing.Labels, desired.Labels) {
		return true
	}

	if !mapContains(existing.Annotations, desired.Annotations) {
		return true
	}

	return false
}

func mapContains(have, want map[string]string) bool {
	if len(want) == 0 {
		return true
	}
	for k, v := range want {
		if have[k] != v {
			return false
		}
	}
	return true
}

func mergeStringMap(existing, desired map[string]string) map[string]string {
	out := make(map[string]string, len(existing)+len(desired))
	for k, v := range existing {
		out[k] = v
	}
	for k, v := range desired {
		out[k] = v
	}
	return out
}

func waitForCRDReady(ctx context.Context, client apiextensionsclient.Interface, name string) error {
	return wait.PollUntilContextTimeout(ctx, appCRDReadyInterval, appCRDReadyTimeout, true, func(ctx context.Context) (bool, error) {
		crd, err := client.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}

		for _, cond := range crd.Status.Conditions {
			if cond.Type == apiextensionsv1.Established && cond.Status == apiextensionsv1.ConditionTrue {
				return true, nil
			}
			if cond.Type == apiextensionsv1.NamesAccepted && cond.Status == apiextensionsv1.ConditionFalse {
				return false, fmt.Errorf("names not accepted: %s", cond.Reason)
			}
		}
		return false, nil
	})
}
