package gitops

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func Clone(repoURL, destinationRoot, dirName string) (string, string, error) {
	source, err := normalizeCloneSource(repoURL)
	if err != nil {
		return "", "", err
	}

	root := strings.TrimSpace(destinationRoot)
	if root == "" {
		return "", "", errors.New("destination root cannot be empty")
	}
	root = filepath.Clean(root)

	name := strings.TrimSpace(dirName)
	if name == "" {
		name = DefaultCloneDirName(source)
	}
	if err := validateCloneFolderName(name); err != nil {
		return filepath.Join(root, name), "", err
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		return filepath.Join(root, name), "", err
	}

	targetPath := filepath.Join(root, name)
	if _, err := os.Stat(targetPath); err == nil {
		return targetPath, "", fmt.Errorf("destination already exists: %s", targetPath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return targetPath, "", err
	}

	output, err := runGitCommand("", "clone", source, targetPath)
	return targetPath, output, err
}

func DefaultCloneDirName(source string) string {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return ""
	}

	if normalized, err := normalizeCloneSource(trimmed); err == nil {
		trimmed = normalized
	}

	trimmed = strings.TrimSuffix(strings.TrimRight(trimmed, "/\\"), ".git")
	if trimmed == "" {
		return ""
	}

	if strings.HasPrefix(trimmed, "git@") {
		if idx := strings.LastIndexAny(trimmed, "/:"); idx >= 0 && idx+1 < len(trimmed) {
			return strings.TrimSpace(trimmed[idx+1:])
		}
	}

	if parsed, err := url.Parse(trimmed); err == nil && parsed.Scheme != "" {
		name := path.Base(strings.Trim(parsed.Path, "/"))
		if name != "." && name != "/" {
			return name
		}
	}

	name := path.Base(strings.ReplaceAll(trimmed, "\\", "/"))
	switch name {
	case "", ".", "/":
		return ""
	default:
		return name
	}
}

func normalizeCloneSource(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", errors.New("repository URL cannot be empty")
	}

	if _, err := os.Stat(trimmed); err == nil {
		return filepath.Clean(trimmed), nil
	}

	switch {
	case strings.Contains(trimmed, "://"):
		return trimmed, nil
	case strings.HasPrefix(trimmed, "git@"):
		return trimmed, nil
	case strings.HasPrefix(trimmed, "github.com/"):
		return "https://" + trimmed, nil
	case filepath.IsAbs(trimmed):
		return trimmed, nil
	case filepath.VolumeName(trimmed) != "":
		return trimmed, nil
	case strings.HasPrefix(trimmed, "."):
		return trimmed, nil
	case strings.HasPrefix(trimmed, "~"):
		return trimmed, nil
	}

	parts := strings.Split(trimmed, "/")
	if len(parts) == 2 && parts[0] != "" && parts[1] != "" && !strings.Contains(trimmed, "\\") && !strings.Contains(trimmed, ":") {
		return "https://github.com/" + strings.TrimSuffix(trimmed, ".git") + ".git", nil
	}

	return trimmed, nil
}

func validateCloneFolderName(name string) error {
	trimmed := strings.TrimSpace(name)
	switch trimmed {
	case "":
		return errors.New("folder name cannot be empty")
	case ".", "..":
		return errors.New("folder name must be a single directory name")
	}

	if strings.Contains(trimmed, "/") || strings.Contains(trimmed, "\\") {
		return errors.New("folder name must be a single directory name")
	}
	if filepath.Base(trimmed) != trimmed {
		return errors.New("folder name must be a single directory name")
	}

	return nil
}
