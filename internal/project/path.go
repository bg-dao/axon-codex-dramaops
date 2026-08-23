package project

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var safeIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,127}$`)

func ValidateID(id string) error {
	if !safeIDPattern.MatchString(id) {
		return fmt.Errorf("invalid id %q", id)
	}
	return nil
}

// ResolveRelative resolves rel inside root and rejects absolute paths, parent
// traversal, and every symlink component. SceneOps intentionally treats
// project-internal symlinks as untrusted so a project cannot escape its root.
func ResolveRelative(root, rel string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("project root is required")
	}
	if strings.TrimSpace(rel) == "" || filepath.IsAbs(rel) {
		return "", fmt.Errorf("path must be a non-empty relative path: %q", rel)
	}
	clean := filepath.Clean(rel)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes project root: %q", rel)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve project root: %w", err)
	}
	rootInfo, err := os.Lstat(absRoot)
	if err != nil {
		return "", fmt.Errorf("inspect project root: %w", err)
	}
	if rootInfo.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("project root cannot be a symlink")
	}

	current := absRoot
	for _, component := range strings.Split(clean, string(filepath.Separator)) {
		current = filepath.Join(current, component)
		info, statErr := os.Lstat(current)
		if statErr == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return "", fmt.Errorf("symlink paths are not allowed: %q", rel)
			}
			continue
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return "", fmt.Errorf("inspect path %q: %w", rel, statErr)
		}
	}

	resolved := filepath.Join(absRoot, clean)
	relCheck, err := filepath.Rel(absRoot, resolved)
	if err != nil || relCheck == ".." || strings.HasPrefix(relCheck, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes project root: %q", rel)
	}
	return resolved, nil
}
