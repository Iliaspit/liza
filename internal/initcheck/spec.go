package initcheck

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/liza-mas/liza/internal/gitenv"
)

// EnsureSpecCommittedClean verifies that specPath is a repo-local file whose
// committed content at HEAD matches the index and working tree.
func EnsureSpecCommittedClean(projectRoot, specPath string) (string, error) {
	repoRel, err := repoRelativePath(projectRoot, specPath)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(filepath.Join(projectRoot, repoRel))
	if err != nil {
		return "", fmt.Errorf("spec file does not exist: %s", specPath)
	}
	if info.IsDir() {
		return "", fmt.Errorf("spec path is a directory: %s", specPath)
	}

	gitPath := filepath.ToSlash(repoRel)
	if _, err := gitenv.CombinedOutput(projectRoot, "ls-files", "--error-unmatch", "--", gitPath); err != nil {
		return "", fmt.Errorf("spec file must be fully committed before liza init: %s", gitPath)
	}
	if _, err := gitenv.CombinedOutput(projectRoot, "cat-file", "-e", "HEAD:"+gitPath); err != nil {
		return "", fmt.Errorf("spec file must be fully committed before liza init: %s", gitPath)
	}
	if changed, err := gitDiffHasChanges(projectRoot, "--cached", "--", gitPath); err != nil {
		return "", err
	} else if changed {
		return "", fmt.Errorf("spec file has staged changes; commit them before liza init: %s", gitPath)
	}
	if changed, err := gitDiffHasChanges(projectRoot, "--", gitPath); err != nil {
		return "", err
	} else if changed {
		return "", fmt.Errorf("spec file has unstaged changes; commit them before liza init: %s", gitPath)
	}

	return gitPath, nil
}

func repoRelativePath(projectRoot, specPath string) (string, error) {
	if projectRoot == "" {
		return "", fmt.Errorf("project root is empty")
	}
	if specPath == "" {
		return "", fmt.Errorf("spec path is empty")
	}

	rootAbs, err := filepath.Abs(projectRoot)
	if err != nil {
		return "", fmt.Errorf("failed to resolve project root: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return "", fmt.Errorf("failed to resolve project root symlinks: %w", err)
	}
	rootAbs = resolvedRoot

	specAbs := specPath
	if !filepath.IsAbs(specAbs) {
		specAbs = filepath.Join(rootAbs, specAbs)
	}
	specAbs, err = filepath.Abs(specAbs)
	if err != nil {
		return "", fmt.Errorf("failed to resolve spec path: %w", err)
	}
	resolvedSpec, err := filepath.EvalSymlinks(specAbs)
	if err != nil {
		if _, lstatErr := os.Lstat(specAbs); lstatErr == nil {
			return "", fmt.Errorf("failed to resolve spec path symlinks: %w", err)
		}
	} else {
		specAbs = resolvedSpec
	}

	rel, err := filepath.Rel(rootAbs, specAbs)
	if err != nil {
		return "", fmt.Errorf("failed to resolve spec path relative to repo: %w", err)
	}
	cleanRel := filepath.Clean(rel)
	if cleanRel == "." || cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) || filepath.IsAbs(cleanRel) {
		return "", fmt.Errorf("spec file must be inside the git repository: %s", specPath)
	}
	return cleanRel, nil
}

func gitDiffHasChanges(projectRoot string, args ...string) (bool, error) {
	fullArgs := append([]string{"diff", "--quiet"}, args...)
	if _, err := gitenv.CombinedOutput(projectRoot, fullArgs...); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return true, nil
		}
		return false, fmt.Errorf("failed to check spec file git status: %w", err)
	}
	return false, nil
}
