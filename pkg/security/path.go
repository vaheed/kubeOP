package security

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	ErrPathEscape     = errors.New("path escapes repository root")
	ErrPathDisallowed = errors.New("path contains disallowed segments")
)

func CleanRoot(root string) (string, error) {
	trimmed := strings.TrimSpace(root)
	if trimmed == "" {
		return "", fmt.Errorf("root is required")
	}
	cleaned := filepath.Clean(trimmed)
	resolved, err := filepath.EvalSymlinks(cleaned)
	if err != nil {
		return "", fmt.Errorf("resolve root: %w", err)
	}
	if !filepath.IsAbs(resolved) {
		return "", fmt.Errorf("root must be absolute")
	}
	return resolved, nil
}

func NormalizeRepoPath(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", nil
	}
	if strings.Contains(trimmed, "\\") {
		return "", fmt.Errorf("%w: backslashes not permitted", ErrPathDisallowed)
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "%2e") {
		return "", fmt.Errorf("%w: encoded traversal not permitted", ErrPathDisallowed)
	}
	for _, r := range trimmed {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("%w: control characters not permitted", ErrPathDisallowed)
		}
	}
	if strings.Contains(trimmed, ":") {
		return "", fmt.Errorf("%w: colon not permitted", ErrPathDisallowed)
	}
	cleaned := filepath.Clean(filepath.FromSlash(trimmed))
	if cleaned == "." {
		return "", nil
	}
	if filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("%w: absolute path %s", ErrPathDisallowed, cleaned)
	}
	if vol := filepath.VolumeName(cleaned); vol != "" {
		return "", fmt.Errorf("%w: volume %s not permitted", ErrPathDisallowed, vol)
	}
	if strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) || cleaned == ".." {
		return "", fmt.Errorf("%w: traversal detected", ErrPathDisallowed)
	}
	return cleaned, nil
}

func WithinRepo(root, input string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("root is required")
	}
	candidate := strings.TrimSpace(input)
	if candidate == "" {
		return root, nil
	}
	var joined string
	if filepath.IsAbs(candidate) {
		joined = filepath.Clean(candidate)
	} else {
		joined = filepath.Join(root, filepath.Clean(filepath.FromSlash(candidate)))
	}
	resolved, err := filepath.EvalSymlinks(joined)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, resolved)
	if err != nil {
		return "", err
	}
	rel = filepath.ToSlash(rel)
	if rel == ".." || strings.HasPrefix(rel, "../") {
		return "", ErrPathEscape
	}
	return resolved, nil
}

func EnsureRegularFile(info fs.FileInfo) error {
	if info == nil {
		return fmt.Errorf("file info required")
	}
	mode := info.Mode()
	if mode.IsRegular() {
		return nil
	}
	if mode.IsDir() {
		return fmt.Errorf("%w: expected file got directory", ErrPathDisallowed)
	}
	if mode&fs.ModeSymlink != 0 {
		return nil
	}
	if mode&fs.ModeDevice != 0 || mode&fs.ModeNamedPipe != 0 || mode&fs.ModeSocket != 0 {
		return fmt.Errorf("%w: special files not permitted", ErrPathDisallowed)
	}
	return nil
}

func RejectWindowsDrive(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	if len(path) >= 2 {
		c := path[0]
		if path[1] == ':' && ((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')) {
			return fmt.Errorf("%w: drive letters not permitted", ErrPathDisallowed)
		}
	}
	return nil
}

func DecodePathSegment(seg string) (string, error) {
	decoded, err := url.PathUnescape(seg)
	if err != nil {
		return "", err
	}
	if strings.Contains(decoded, "..") {
		return "", fmt.Errorf("%w: traversal", ErrPathDisallowed)
	}
	return decoded, nil
}
