package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/filelock"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestProjectLifecycleLockLivesOutsideCleanupTargets(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectRoot)

	if err := WithProjectLifecycleSharedLock(projectRoot, "test", func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	lockName := strings.TrimPrefix(paths.ProjectDirName(), ".") + "-project-lifecycle.lock"
	lockPath := filepath.Join(projectRoot, paths.GitDirName, lockName)
	if _, err := os.Stat(lockPath); err != nil {
		t.Fatalf("project lifecycle lock missing from Git metadata: %v", err)
	}
	for _, cleanupDir := range []string{paths.New(projectRoot).LizaDir(), filepath.Join(projectRoot, paths.WorktreesDirName)} {
		rel, err := filepath.Rel(cleanupDir, lockPath)
		if err != nil {
			t.Fatal(err)
		}
		if rel == "." || (!filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))) {
			t.Fatalf("project lifecycle lock %s is inside cleanup target %s", lockPath, cleanupDir)
		}
	}
}

func TestProjectLifecycleLockSupportsLinkedWorktreeRoot(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectRoot)
	testhelpers.CreateTestWorktree(t, projectRoot, "linked-project")
	linkedRoot := filepath.Join(projectRoot, paths.WorktreesDirName, "linked-project")

	if err := WithProjectLifecycleSharedLock(linkedRoot, "test", func() error { return nil }); err != nil {
		t.Fatal(err)
	}
	gitDir := testhelpers.MustGit(t, linkedRoot, "rev-parse", "--absolute-git-dir")
	lockName := strings.TrimPrefix(paths.ProjectDirName(), ".") + "-project-lifecycle.lock"
	if _, err := os.Stat(filepath.Join(gitDir, lockName)); err != nil {
		t.Fatalf("project lifecycle lock missing from linked-worktree Git metadata: %v", err)
	}
}

func TestProjectLifecycleSharedLockPreservesNonRepositoryValidation(t *testing.T) {
	t.Parallel()

	called := false
	wantErr := os.ErrInvalid
	err := WithProjectLifecycleSharedLock(filepath.Join(t.TempDir(), "missing"), "test", func() error {
		called = true
		return wantErr
	})
	if !called {
		t.Fatal("shared lifecycle callback was not called for non-repository input")
	}
	if err != wantErr {
		t.Fatalf("WithProjectLifecycleSharedLock() error = %v, want %v", err, wantErr)
	}
}

func TestProjectLifecycleLockTimeoutIsActionable(t *testing.T) {
	t.Parallel()

	projectRoot := t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectRoot)

	lock, err := projectLifecycleLock(projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	held := make(chan struct{})
	release := make(chan struct{})
	holderDone := make(chan error, 1)
	go func() {
		holderDone <- withProjectLifecycleLock(lock, "worktree-create", true, time.Second, func() error {
			close(held)
			<-release
			return nil
		})
	}()
	select {
	case <-held:
	case holderErr := <-holderDone:
		t.Fatalf("shared lifecycle holder failed before acquisition: %v", holderErr)
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for shared lifecycle holder")
	}

	callbackCalled := false
	err = withProjectLifecycleLock(lock, "project-cleanup", false, time.Millisecond, func() error {
		callbackCalled = true
		return nil
	})
	close(release)
	if holderErr := <-holderDone; holderErr != nil {
		t.Fatalf("shared lifecycle holder failed: %v", holderErr)
	}

	if callbackCalled {
		t.Fatal("exclusive lifecycle callback ran without acquiring the lock")
	}
	if !filelock.IsLockErrorType(err, filelock.LockErrorTimeout) {
		t.Fatalf("lifecycle contention error = %T %v, want timeout classification", err, err)
	}
	for _, want := range []string{
		`project lifecycle operation "project-cleanup"`,
		"worktree provisioning/recovery operations are still running",
		"wait for those operations to finish and retry",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("lifecycle contention error %q missing %q", err, want)
		}
	}
}
