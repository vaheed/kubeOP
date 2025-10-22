package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5/plumbing/transport"
	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	gitssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"go.uber.org/zap"
	cryptossh "golang.org/x/crypto/ssh"
	"kubeop/internal/delivery"
	"kubeop/internal/logging"
	"kubeop/internal/store"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
)

const (
	gitModeManifests = "manifests"
	gitModeKustomize = "kustomize"
)

func (s *Service) prepareGitPlan(spec *AppGitSpec) (*gitSourcePlan, error) {
	if spec == nil {
		return nil, nil
	}
	urlStr := strings.TrimSpace(spec.URL)
	if urlStr == "" {
		return nil, errors.New("git.url is required")
	}
	if err := s.validateGitURL(urlStr); err != nil {
		return nil, err
	}
	ref := strings.TrimSpace(spec.Ref)
	checkoutHash := ""
	referenceName := ""
	if ref == "" {
		ref = "refs/heads/main"
	}
	switch {
	case strings.HasPrefix(ref, "refs/"):
		referenceName = ref
	case isHexHash(ref):
		checkoutHash = ref
	default:
		referenceName = fmt.Sprintf("refs/heads/%s", ref)
	}
	mode := strings.ToLower(strings.TrimSpace(spec.Mode))
	if mode == "" {
		mode = gitModeManifests
	}
	switch mode {
	case gitModeManifests, gitModeKustomize:
	default:
		return nil, fmt.Errorf("git.mode must be %q or %q", gitModeManifests, gitModeKustomize)
	}
	path := strings.TrimSpace(spec.Path)
	if path != "" {
		clean := filepath.Clean(path)
		clean = strings.TrimPrefix(clean, string(filepath.Separator))
		if clean == "." {
			clean = ""
		}
		if clean != "" && (strings.HasPrefix(clean, "..") || strings.Contains(clean, ".."+string(filepath.Separator))) {
			return nil, errors.New("git.path must remain within the repository")
		}
		path = clean
	}
	credentialID := strings.TrimSpace(spec.CredentialID)
	return &gitSourcePlan{
		URL:             urlStr,
		Ref:             ref,
		Path:            path,
		Mode:            mode,
		CredentialID:    credentialID,
		InsecureSkipTLS: spec.InsecureSkipTLS,
		referenceName:   referenceName,
		checkoutHash:    checkoutHash,
	}, nil
}

func (s *Service) validateGitURL(raw string) error {
	if strings.HasPrefix(raw, "git@") {
		// git@github.com:org/repo.git style URLs are allowed as-is.
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid git url: %w", err)
	}
	scheme := strings.ToLower(parsed.Scheme)
	switch scheme {
	case "https", "ssh", "git+ssh":
		return nil
	case "file":
		if s.cfg == nil || !s.cfg.AllowGitFileProtocol {
			return errors.New("file:// git urls are disabled")
		}
		if parsed.Path == "" {
			return errors.New("file git url requires path")
		}
		return nil
	default:
		return fmt.Errorf("unsupported git url scheme %q", scheme)
	}
}

func isHexHash(ref string) bool {
	if len(ref) != 40 {
		return false
	}
	for _, r := range ref {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') && (r < 'A' || r > 'F') {
			return false
		}
	}
	return true
}

func (s *Service) renderGitSource(ctx context.Context, project store.Project, plan *gitSourcePlan) ([]string, string, []string, error) {
	if plan == nil {
		return nil, "", nil, errors.New("git source not configured")
	}
	var auth transport.AuthMethod
	if plan.CredentialID != "" {
		var err error
		auth, err = s.gitCredentialAuth(ctx, project, plan.CredentialID, plan.URL)
		if err != nil {
			return nil, "", nil, err
		}
	}
	checkout, err := delivery.CheckoutGit(ctx, delivery.GitCheckoutOptions{
		URL:             plan.URL,
		ReferenceName:   plan.referenceName,
		CheckoutHash:    plan.checkoutHash,
		Path:            plan.Path,
		Auth:            auth,
		InsecureSkipTLS: plan.InsecureSkipTLS,
	})
	if err != nil {
		return nil, "", nil, err
	}
	defer checkout.Cleanup()
	plan.Commit = checkout.Commit
	switch plan.Mode {
	case gitModeKustomize:
		rendered, err := renderKustomize(checkout.BasePath)
		if err != nil {
			return nil, "", nil, err
		}
		if strings.TrimSpace(rendered) == "" {
			return nil, "", nil, errors.New("kustomize rendered empty output")
		}
		return []string{rendered}, checkout.Commit, nil, nil
	default:
		docs, err := delivery.LoadManifests(checkout.RepoRoot, checkout.BasePath, checkout.Info)
		if err != nil {
			return nil, "", nil, err
		}
		if len(docs) == 0 {
			return nil, "", nil, errors.New("no YAML manifests found in git path")
		}
		return docs, checkout.Commit, nil, nil
	}
}

func renderKustomize(path string) (string, error) {
	fsys := filesys.MakeFsOnDisk()
	opts := krusty.MakeDefaultOptions()
	k := krusty.MakeKustomizer(opts)
	resMap, err := k.Run(fsys, path)
	if err != nil {
		return "", fmt.Errorf("kustomize build: %w", err)
	}
	by, err := resMap.AsYaml()
	if err != nil {
		return "", fmt.Errorf("kustomize yaml: %w", err)
	}
	return string(by), nil
}

func (s *Service) gitCredentialAuth(ctx context.Context, project store.Project, credentialID, repoURL string) (transport.AuthMethod, error) {
	out, err := s.GetGitCredential(ctx, credentialID)
	if err != nil {
		return nil, err
	}
	switch out.ScopeType {
	case CredentialScopeProject:
		if out.ScopeID != project.ID {
			return nil, errors.New("git credential not accessible to project")
		}
	case CredentialScopeUser:
		if out.ScopeID != project.UserID {
			return nil, errors.New("git credential not accessible to project user")
		}
	default:
		return nil, errors.New("git credential scope unsupported")
	}
	authType := strings.ToUpper(strings.TrimSpace(out.AuthType))
	switch authType {
	case "TOKEN":
		token := strings.TrimSpace(out.Secret.Token)
		if token == "" {
			return nil, errors.New("git credential token is empty")
		}
		username := strings.TrimSpace(out.Username)
		if username == "" {
			username = "git"
		}
		return &githttp.BasicAuth{Username: username, Password: token}, nil
	case "BASIC":
		username := strings.TrimSpace(out.Username)
		password := strings.TrimSpace(out.Secret.Password)
		if username == "" || password == "" {
			return nil, errors.New("git credential username/password required for BASIC authType")
		}
		return &githttp.BasicAuth{Username: username, Password: password}, nil
	case "SSH":
		key := strings.TrimSpace(out.Secret.PrivateKey)
		if key == "" {
			return nil, errors.New("git credential privateKey required for SSH authType")
		}
		username := strings.TrimSpace(out.Username)
		if username == "" {
			username = "git"
		}
		auth, err := gitssh.NewPublicKeys(username, []byte(key), strings.TrimSpace(out.Secret.Passphrase))
		if err != nil {
			return nil, fmt.Errorf("git credential ssh auth: %w", err)
		}
		auth.HostKeyCallbackHelper = gitssh.HostKeyCallbackHelper{HostKeyCallback: cryptossh.InsecureIgnoreHostKey()}
		return auth, nil
	default:
		return nil, fmt.Errorf("git credential authType %s unsupported", out.AuthType)
	}
}

func (s *Service) logGitFetch(projectID, repoURL string, commit string) {
	logger := logging.ProjectLogger(projectID)
	logger.Info("git_source_rendered", zap.String("repo", repoURL), zap.String("commit", commit))
}
