package service

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"

	"go.uber.org/zap"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"kubeop/internal/crypto"
	"kubeop/internal/logging"
)

type KubeconfigEnsureInput struct {
	UserID    string
	ProjectID string
	ClusterID string
}

type KubeconfigEnsureOutput struct {
	ID             string `json:"id"`
	ClusterID      string `json:"cluster_id"`
	UserID         string `json:"user_id"`
	ProjectID      string `json:"project_id,omitempty"`
	Namespace      string `json:"namespace"`
	ServiceAccount string `json:"service_account"`
	SecretName     string `json:"secret_name"`
	KubeconfigB64  string `json:"kubeconfig_b64"`
}

type KubeconfigRotateOutput struct {
	ID            string `json:"id"`
	SecretName    string `json:"secret_name"`
	KubeconfigB64 string `json:"kubeconfig_b64"`
}

func (s *Service) EnsureKubeconfigBinding(ctx context.Context, in KubeconfigEnsureInput) (KubeconfigEnsureOutput, error) {
	if in.UserID == "" {
		return KubeconfigEnsureOutput{}, errors.New("userId is required")
	}
	if in.ProjectID == "" && in.ClusterID == "" {
		return KubeconfigEnsureOutput{}, errors.New("clusterId is required when projectId is not provided")
	}
	if in.ProjectID != "" {
		return s.ensureProjectKubeconfig(ctx, in)
	}
	return s.ensureUserKubeconfig(ctx, in)
}

func (s *Service) ensureUserKubeconfig(ctx context.Context, in KubeconfigEnsureInput) (KubeconfigEnsureOutput, error) {
	us, enc, err := s.st.GetUserSpace(ctx, in.UserID, in.ClusterID)
	if err != nil || len(enc) == 0 {
		if _, err := s.provisionUserSpace(ctx, in.UserID, in.ClusterID); err != nil {
			return KubeconfigEnsureOutput{}, err
		}
		us, enc, err = s.st.GetUserSpace(ctx, in.UserID, in.ClusterID)
		if err != nil {
			return KubeconfigEnsureOutput{}, err
		}
	}
	if len(enc) == 0 {
		if _, err := s.RenewUserKubeconfig(ctx, in.UserID, in.ClusterID); err != nil {
			return KubeconfigEnsureOutput{}, err
		}
	}
	rec, recEnc, err := s.st.GetKubeconfigByUserScope(ctx, in.ClusterID, us.Namespace, in.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if _, err := s.RenewUserKubeconfig(ctx, in.UserID, in.ClusterID); err != nil {
				return KubeconfigEnsureOutput{}, err
			}
			rec, recEnc, err = s.st.GetKubeconfigByUserScope(ctx, in.ClusterID, us.Namespace, in.UserID)
		}
		if err != nil {
			return KubeconfigEnsureOutput{}, err
		}
	}
	if len(recEnc) == 0 {
		if _, err := s.RenewUserKubeconfig(ctx, in.UserID, in.ClusterID); err != nil {
			return KubeconfigEnsureOutput{}, err
		}
		rec, recEnc, err = s.st.GetKubeconfigByUserScope(ctx, in.ClusterID, us.Namespace, in.UserID)
		if err != nil {
			return KubeconfigEnsureOutput{}, err
		}
	}
	kc, err := crypto.DecryptAESGCM(recEnc, s.encKey)
	if err != nil {
		return KubeconfigEnsureOutput{}, err
	}
	return KubeconfigEnsureOutput{
		ID:             rec.ID,
		ClusterID:      rec.ClusterID,
		UserID:         in.UserID,
		Namespace:      rec.Namespace,
		ServiceAccount: rec.ServiceAccount,
		SecretName:     rec.SecretName,
		KubeconfigB64:  base64.StdEncoding.EncodeToString(kc),
	}, nil
}

func (s *Service) ensureProjectKubeconfig(ctx context.Context, in KubeconfigEnsureInput) (KubeconfigEnsureOutput, error) {
	p, _, _, err := s.st.GetProject(ctx, in.ProjectID)
	if err != nil {
		return KubeconfigEnsureOutput{}, err
	}
	if p.UserID != "" && p.UserID != in.UserID {
		return KubeconfigEnsureOutput{}, errors.New("project does not belong to user")
	}
	rec, recEnc, err := s.st.GetKubeconfigByProject(ctx, in.ProjectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if _, err := s.RenewProjectKubeconfig(ctx, in.ProjectID); err != nil {
				return KubeconfigEnsureOutput{}, err
			}
			rec, recEnc, err = s.st.GetKubeconfigByProject(ctx, in.ProjectID)
		}
		if err != nil {
			return KubeconfigEnsureOutput{}, err
		}
	}
	if len(recEnc) == 0 {
		if _, err := s.RenewProjectKubeconfig(ctx, in.ProjectID); err != nil {
			return KubeconfigEnsureOutput{}, err
		}
		rec, recEnc, err = s.st.GetKubeconfigByProject(ctx, in.ProjectID)
		if err != nil {
			return KubeconfigEnsureOutput{}, err
		}
	}
	kc, err := crypto.DecryptAESGCM(recEnc, s.encKey)
	if err != nil {
		return KubeconfigEnsureOutput{}, err
	}
	return KubeconfigEnsureOutput{
		ID:             rec.ID,
		ClusterID:      rec.ClusterID,
		UserID:         p.UserID,
		ProjectID:      p.ID,
		Namespace:      rec.Namespace,
		ServiceAccount: rec.ServiceAccount,
		SecretName:     rec.SecretName,
		KubeconfigB64:  base64.StdEncoding.EncodeToString(kc),
	}, nil
}

func (s *Service) RotateKubeconfigByID(ctx context.Context, id string) (KubeconfigRotateOutput, error) {
	rec, _, err := s.st.GetKubeconfigByID(ctx, id)
	if err != nil {
		return KubeconfigRotateOutput{}, err
	}
	previousSecret := rec.SecretName
	if rec.ProjectID != nil {
		out, err := s.RenewProjectKubeconfig(ctx, *rec.ProjectID)
		if err != nil {
			return KubeconfigRotateOutput{}, err
		}
		if err := s.deleteSecret(ctx, rec.ClusterID, rec.Namespace, previousSecret); err != nil {
			logging.ProjectLogger(*rec.ProjectID).Warn("rotate_delete_old_secret", zapError(err), zap.String("secret", previousSecret))
		}
		updated, _, err := s.st.GetKubeconfigByID(ctx, id)
		if err != nil {
			return KubeconfigRotateOutput{}, err
		}
		return KubeconfigRotateOutput{ID: updated.ID, SecretName: updated.SecretName, KubeconfigB64: out.KubeconfigB64}, nil
	}
	out, err := s.RenewUserKubeconfig(ctx, rec.UserID, rec.ClusterID)
	if err != nil {
		return KubeconfigRotateOutput{}, err
	}
	if err := s.deleteSecret(ctx, rec.ClusterID, rec.Namespace, previousSecret); err != nil {
		s.logger.Warn("rotate_delete_old_secret", zapError(err), zap.String("secret", previousSecret))
	}
	updated, _, err := s.st.GetKubeconfigByID(ctx, id)
	if err != nil {
		return KubeconfigRotateOutput{}, err
	}
	return KubeconfigRotateOutput{ID: updated.ID, SecretName: updated.SecretName, KubeconfigB64: out.KubeconfigB64}, nil
}

func (s *Service) DeleteKubeconfigBinding(ctx context.Context, id string) error {
	rec, _, err := s.st.GetKubeconfigByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.deleteSecret(ctx, rec.ClusterID, rec.Namespace, rec.SecretName); err != nil {
		return err
	}
	if rec.ProjectID != nil {
		if err := s.st.UpdateProjectKubeconfig(ctx, *rec.ProjectID, nil); err != nil {
			return err
		}
	} else {
		if us, _, err := s.st.GetUserSpace(ctx, rec.UserID, rec.ClusterID); err == nil {
			if err := s.st.UpdateUserSpaceKubeconfig(ctx, us.ID, nil); err != nil {
				return err
			}
		}
	}
	if err := s.st.DeleteKubeconfigRecord(ctx, id); err != nil {
		return err
	}
	count, err := s.st.CountKubeconfigsByServiceAccount(ctx, rec.Namespace, rec.ServiceAccount)
	if err == nil && count == 0 {
		_ = s.deleteServiceAccount(ctx, rec.ClusterID, rec.Namespace, rec.ServiceAccount)
	}
	return nil
}

func (s *Service) deleteSecret(ctx context.Context, clusterID, namespace, name string) error {
	if namespace == "" || name == "" {
		return errors.New("secret namespace and name required")
	}
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, clusterID) }
	cs, err := s.km.GetClientset(ctx, clusterID, loader)
	if err != nil {
		return err
	}
	err = cs.CoreV1().Secrets(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func (s *Service) deleteServiceAccount(ctx context.Context, clusterID, namespace, saName string) error {
	loader := func(ctx context.Context) ([]byte, error) { return s.DecryptClusterKubeconfig(ctx, clusterID) }
	cs, err := s.km.GetClientset(ctx, clusterID, loader)
	if err != nil {
		return err
	}
	err = cs.CoreV1().ServiceAccounts(namespace).Delete(ctx, saName, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	return nil
}

func zapError(err error) zap.Field {
	return zap.Error(err)
}
