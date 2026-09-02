package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/liza-mas/liza/internal/filelock"
	"github.com/liza-mas/liza/internal/paths"
)

// projectFileLock returns a brand-aware lock stored in the repository's Git
// metadata, outside project runtime directories that cleanup may remove.
func projectFileLock(projectRoot, purpose string) (*filelock.FileLock, error) {
	root, err := filepath.Abs(projectRoot)
	if err != nil {
		return nil, fmt.Errorf("resolve project root for %s lock: %w", purpose, err)
	}
	root, err = filepath.EvalSymlinks(filepath.Clean(root))
	if err != nil {
		return nil, fmt.Errorf("resolve project root symlinks for %s lock: %w", purpose, err)
	}

	gitMarker := filepath.Join(root, paths.GitDirName)
	info, err := os.Stat(gitMarker)
	if err != nil {
		return nil, fmt.Errorf("inspect Git metadata for %s lock: %w", purpose, err)
	}

	gitDir := gitMarker
	if !info.IsDir() {
		contents, err := os.ReadFile(gitMarker)
		if err != nil {
			return nil, fmt.Errorf("read Git metadata pointer for %s lock: %w", purpose, err)
		}
		pointer := strings.TrimSpace(string(contents))
		const prefix = "gitdir:"
		if !strings.HasPrefix(pointer, prefix) {
			return nil, fmt.Errorf("Git metadata for %s lock is neither a directory nor a gitdir pointer: %s", purpose, gitMarker)
		}
		gitDir = strings.TrimSpace(strings.TrimPrefix(pointer, prefix))
		if gitDir == "" {
			return nil, fmt.Errorf("Git metadata pointer for %s lock is empty: %s", purpose, gitMarker)
		}
		if !filepath.IsAbs(gitDir) {
			gitDir = filepath.Join(filepath.Dir(gitMarker), gitDir)
		}
		gitDir, err = filepath.EvalSymlinks(filepath.Clean(gitDir))
		if err != nil {
			return nil, fmt.Errorf("resolve Git metadata pointer for %s lock: %w", purpose, err)
		}
	}
	info, err = os.Stat(gitDir)
	if err != nil {
		return nil, fmt.Errorf("inspect Git directory for %s lock: %w", purpose, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("git directory for %s lock is not a directory: %s", purpose, gitDir)
	}

	lockName := strings.TrimPrefix(paths.ProjectDirName(), ".") + "-" + purpose
	return filelock.New(filepath.Join(gitDir, lockName)), nil
}
