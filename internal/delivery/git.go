package delivery

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
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
	repoRoot, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("resolve git repo root: %w", err)
	}
	base := repoRoot
	if validatedPath != "" {
		resolved, err := resolveRepoPath(repoRoot, validatedPath)
		if err != nil {
			cleanup()
			return nil, fmt.Errorf("resolve git path %s: %w", validatedPath, err)
		}
		base = resolved
	}
	if err := ensureWithinRepo(repoRoot, base); err != nil {
		cleanup()
		return nil, fmt.Errorf("git path %s escapes repo: %w", base, err)
	}
	info, err := os.Stat(base)
	if err != nil {
		cleanup()
		return nil, fmt.Errorf("git path %s: %w", validatedPath, err)
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
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", nil
	}
	cleaned := filepath.Clean(trimmed)
	if cleaned == "." {
		return "", nil
	}
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("path %s must be relative", trimmed)
	}
	if vol := filepath.VolumeName(cleaned); vol != "" {
		return "", fmt.Errorf("path %s must not specify a volume", trimmed)
	}
	if len(cleaned) >= 2 {
		c := cleaned[0]
		if cleaned[1] == ':' && ((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')) {
			return "", fmt.Errorf("path %s must not specify a drive", trimmed)
		}
	}
	normalised := filepath.ToSlash(cleaned)
	if normalised == ".." || strings.HasPrefix(normalised, "../") {
		return "", fmt.Errorf("path %s must stay within repository", trimmed)
	}
	if _, err := pathSegments(trimmed); err != nil {
		return "", err
	}
	return cleaned, nil
}

// resolveRepoPath safely joins the repository root with the validated path, ensuring no escapes.
func resolveRepoPath(root, rel string) (string, error) {
	if rel == "" {
		return root, nil
	}

	segments, err := pathSegments(rel)
	if err != nil {
		return "", err
	}

	candidate := root
	for _, segment := range segments {
		candidate = filepath.Join(candidate, segment)
	}

	resolved, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", err
	}
	if err := ensureWithinRepo(root, resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

func pathSegments(rel string) ([]string, error) {
	if rel == "" {
		return nil, nil
	}

	normalised := filepath.ToSlash(rel)
	parts := strings.Split(normalised, "/")
	segments := make([]string, 0, len(parts))

	for _, part := range parts {
		switch part {
		case "", ".":
			continue
		case "..":
			return nil, fmt.Errorf("path %s must stay within repository", rel)
		}
		if strings.Contains(part, ":") {
			return nil, fmt.Errorf("path segment %q must not contain colons", part)
		}
		for _, r := range part {
			if r < 0x20 || r == 0x7f {
				return nil, fmt.Errorf("path segment %q contains control characters", part)
			}
		}
		segments = append(segments, part)
	}

	return segments, nil
}

// EnsureWithinRepo ensures that a resolved path does not escape the repository root.
func ensureWithinRepo(root, path string) error {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return fmt.Errorf("determine relative path: %w", err)
	}
	rel = filepath.ToSlash(rel)
	if rel == "." {
		return nil
	}
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return fmt.Errorf("path %s escapes repository root", path)
	}
	return nil
}

func sanitizeRepoRoot(root string) (string, error) {
	trimmed := strings.TrimSpace(root)
	if trimmed == "" {
		return "", fmt.Errorf("repo root is required")
	}
	cleaned := filepath.Clean(trimmed)
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		return "", fmt.Errorf("resolve repo root: %w", err)
	}
	if !filepath.IsAbs(resolved) {
		return "", fmt.Errorf("repo root %s must be absolute", resolved)
	}
	return resolved, nil
}

func sanitizeRepoBase(root, base string) (string, error) {
	trimmed := strings.TrimSpace(base)
	if trimmed == "" {
		return root, nil
	}
	candidate := trimmed
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(root, candidate)
	}
	cleaned := filepath.Clean(candidate)
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		return "", fmt.Errorf("resolve git path %s: %w", base, err)
	}
	if err := ensureWithinRepo(root, resolved); err != nil {
		return "", err
	}
	return resolved, nil
}

// LoadManifests walks the checkout and returns YAML documents under the requested base path.
func LoadManifests(root, base string, info fs.FileInfo) ([]string, error) {
	// Normalise and bound the repository inputs so malicious paths cannot escape the checkout
	// when we walk the filesystem (CodeQL flagged the lack of explicit sanitisation).
	sanitizedRoot, err := sanitizeRepoRoot(root)
	if err != nil {
		return nil, fmt.Errorf("sanitize repo root: %w", err)
	}
	sanitizedBase, err := sanitizeRepoBase(sanitizedRoot, base)
	if err != nil {
		return nil, err
	}
	if sanitizedBase != base {
		info, err = os.Stat(sanitizedBase)
		if err != nil {
			return nil, fmt.Errorf("stat sanitized git path: %w", err)
		}
	}
	root = sanitizedRoot
	base = sanitizedBase

	var files []string
	resolve := func(path string) (string, error) {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return "", fmt.Errorf("resolve %s: %w", path, err)
		}
		if err := ensureWithinRepo(root, resolved); err != nil {
			return "", err
		}
		return resolved, nil
	}
	if info.IsDir() {
		err := filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			resolved, err := resolve(path)
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			if !isYAML(path) {
				return nil
			}
			files = append(files, resolved)
			return nil
		})
		if err != nil {
			return nil, err
		}
		sort.Strings(files)
	} else {
		if !isYAML(base) {
			return nil, fmt.Errorf("git path %s is not a YAML file", base)
		}
		resolved, err := resolve(base)
		if err != nil {
			return nil, err
		}
		files = []string{resolved}
	}
	docs := make([]string, 0, len(files))
	for _, file := range files {
		if err := ensureWithinRepo(root, file); err != nil {
			return nil, err
		}
		by, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("read manifest %s: %w", file, err)
		}
		docs = append(docs, string(by))
	}
	return docs, nil
}

func isYAML(path string) bool {
	lower := strings.ToLower(filepath.Ext(path))
	return lower == ".yaml" || lower == ".yml"
}
