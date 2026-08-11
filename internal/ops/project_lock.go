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

	gitDir := filepath.Join(root, paths.GitDirName)
	info, err := os.Stat(gitDir)
	if err != nil {
		return nil, fmt.Errorf("inspect Git directory for %s lock: %w", purpose, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("git directory for %s lock is not a directory: %s", purpose, gitDir)
	}

	lockName := strings.TrimPrefix(paths.ProjectDirName(), ".") + "-" + purpose
	return filelock.New(filepath.Join(gitDir, lockName)), nil
}
