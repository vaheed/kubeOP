package delivery

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"kubeop/pkg/security"
)

// GitCheckoutOptions captures clone parameters for source rendering.
type GitCheckoutOptions struct {
	URL             string
	ReferenceName   string
	CheckoutHash    string
	Path            string
	Auth            transport.AuthMethod
	InsecureSkipTLS bool
}

// GitCheckoutResult returns the resolved repository path and commit.
type GitCheckoutResult struct {
	RepoRoot string
	BasePath string
	Info     fs.FileInfo
	Commit   string
	Cleanup  func() error
}

// CheckoutGit clones the repository to a temporary directory and resolves the requested path.
func CheckoutGit(ctx context.Context, opts GitCheckoutOptions) (*GitCheckoutResult, error) {
	tmpDir, err := os.MkdirTemp("", "kubeop-git-")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	cleanup := func() error { return os.RemoveAll(tmpDir) }
	validatedPath, err := ValidateCheckoutPath(opts.Path)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("invalid git path: %w", err)
	}
	cloneOpts := &git.CloneOptions{
		URL:             opts.URL,
		Depth:           1,
		InsecureSkipTLS: opts.InsecureSkipTLS,
	}
	if opts.Auth != nil {
		cloneOpts.Auth = opts.Auth
	}
	if strings.TrimSpace(opts.ReferenceName) != "" {
		cloneOpts.ReferenceName = plumbing.ReferenceName(opts.ReferenceName)
		cloneOpts.SingleBranch = true
	}
	if strings.TrimSpace(opts.CheckoutHash) != "" {
		cloneOpts.SingleBranch = false
	}
	repo, err := git.PlainCloneContext(ctx, tmpDir, false, cloneOpts)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("clone git repo: %w", err)
	}
	worktree, err := repo.Worktree()
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("git worktree: %w", err)
	}
	if hash := strings.TrimSpace(opts.CheckoutHash); hash != "" {
		h := plumbing.NewHash(hash)
		if err := worktree.Checkout(&git.CheckoutOptions{Hash: h}); err != nil {
			cleanup()
			return nil, fmt.Errorf("checkout %s: %w", hash, err)
		}
	}
	head, err := repo.Head()
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("git head: %w", err)
	}
	repoRoot, err := security.CleanRoot(tmpDir)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("resolve git repo root: %w", err)
	}
	base := repoRoot
	if validatedPath != "" {
		resolved, err := security.WithinRepo(repoRoot, validatedPath)
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("resolve git path %s: %w", validatedPath, err)
		}
		base = resolved
	}
	info, err := os.Lstat(base)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("git path %s: %w", validatedPath, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		resolved, err := security.WithinRepo(repoRoot, base)
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("resolve git path %s: %w", base, err)
		}
		info, err = os.Stat(resolved)
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("git path %s: %w", validatedPath, err)
		}
		base = resolved
	}
	if !info.IsDir() {
		if err := security.EnsureRegularFile(info); err != nil {
			cleanup()
			return nil, fmt.Errorf("git path %s: %w", validatedPath, err)
		}
	}
	return &GitCheckoutResult{
		RepoRoot: repoRoot,
		BasePath: base,
		Info:     info,
		Commit:   head.Hash().String(),
		Cleanup:  cleanup,
	}, nil
}

// ValidateCheckoutPath ensures git checkout paths remain relative to the cloned repository.
func ValidateCheckoutPath(path string) (string, error) {
	normalized, err := security.NormalizeRepoPath(path)
	if err != nil {
		return "", err
	}
	return normalized, nil
}

// LoadManifests walks the checkout and returns YAML documents under the requested base path.
func LoadManifests(root, base string, info fs.FileInfo) ([]string, error) {
	sanitizedRoot, err := security.CleanRoot(root)
	if err != nil {
		return nil, fmt.Errorf("sanitize repo root: %w", err)
	}
	sanitizedBase, err := security.WithinRepo(sanitizedRoot, base)
	if err != nil {
		return nil, err
	}
	baseInfo := info
	if baseInfo == nil || sanitizedBase != base {
		baseInfo, err = os.Lstat(sanitizedBase)
		if err != nil {
			return nil, fmt.Errorf("stat sanitized git path: %w", err)
		}
	}
	if baseInfo.Mode()&os.ModeSymlink != 0 {
		resolved, err := security.WithinRepo(sanitizedRoot, sanitizedBase)
		if err != nil {
			return nil, err
		}
		baseInfo, err = os.Stat(resolved)
		if err != nil {
			return nil, fmt.Errorf("stat resolved git path: %w", err)
		}
		sanitizedBase = resolved
	}

	var files []string
	if baseInfo.IsDir() {
		relStart, err := filepath.Rel(sanitizedRoot, sanitizedBase)
		if err != nil {
			return nil, fmt.Errorf("determine relative base: %w", err)
		}
		if relStart == "" {
			relStart = "."
		}
		walkFS := os.DirFS(sanitizedRoot)
		err = fs.WalkDir(walkFS, filepath.ToSlash(relStart), func(entryPath string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entryPath == "." {
				return nil
			}
			resolved, err := security.WithinRepo(sanitizedRoot, entryPath)
			if err != nil {
				if errors.Is(err, security.ErrPathEscape) {
					if d.IsDir() {
						return fs.SkipDir
					}
					return nil
				}
				return err
			}
			info, err := os.Stat(resolved)
			if err != nil {
				return err
			}
			if info.IsDir() {
				if d.Name() == ".git" {
					return fs.SkipDir
				}
				return nil
			}
			if !isYAML(resolved) {
				return nil
			}
			if err := security.EnsureRegularFile(info); err != nil {
				return err
			}
			files = append(files, resolved)
			return nil
		})
		if err != nil {
			return nil, err
		}
		sort.Strings(files)
	} else {
		if !isYAML(sanitizedBase) {
			return nil, fmt.Errorf("git path %s is not a YAML file", sanitizedBase)
		}
		if err := security.EnsureRegularFile(baseInfo); err != nil {
			return nil, err
		}
		files = []string{sanitizedBase}
	}

	docs := make([]string, 0, len(files))
	for _, file := range files {
		resolved, err := security.WithinRepo(sanitizedRoot, file)
		if err != nil {
			return nil, err
		}
		by, err := os.ReadFile(resolved)
		if err != nil {
			return nil, fmt.Errorf("read manifest %s: %w", resolved, err)
		}
		docs = append(docs, string(by))
	}
	return docs, nil
}

func isYAML(path string) bool {
	lower := strings.ToLower(filepath.Ext(path))
	return lower == ".yaml" || lower == ".yml"
}
