package commands

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	gitops "github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestCleanupProjectCommandDeclinedPreservesTargets(t *testing.T) {
	withTestBrandDirs(t, ".acme", ".acme")
	projectRoot, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	testhelpers.SetupTestGitRepo(t, projectRoot)
	lp := paths.New(projectRoot)
	marker := filepath.Join(lp.LizaDir(), "keep")
	if err := os.MkdirAll(lp.LizaDir(), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	_, err = CleanupProjectCommand(CleanupParams{
		ProjectRoot: projectRoot,
		Stdin:       strings.NewReader("n\n"),
		Stderr:      &stderr,
	})
	if !errors.Is(err, ErrProjectCleanupDeclined) {
		t.Fatalf("CleanupProjectCommand() error = %v, want declined", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("declined cleanup removed marker: %v", err)
	}
	for _, want := range []string{lp.LizaDir(), "permanently deleted", "uncommitted changes"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("cleanup prompt missing %q:\n%s", want, stderr.String())
		}
	}
	if strings.Contains(stderr.String(), ".liza") {
		t.Fatalf("cleanup prompt leaked default runtime directory under custom branding:\n%s", stderr.String())
	}
}

func TestCleanupProjectCommandAutoConfirmDeletesBranchAndWorktree(t *testing.T) {
	projectRoot := t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectRoot)
	gitClient := gitops.New(projectRoot)
	if _, err := gitClient.CreateWorktree("old-task", "HEAD"); err != nil {
		t.Fatal(err)
	}

	var stderr bytes.Buffer
	result, err := CleanupProjectCommand(CleanupParams{
		ProjectRoot: projectRoot,
		Stderr:      &stderr,
		AutoConfirm: true,
	})
	if err != nil {
		t.Fatalf("CleanupProjectCommand() error = %v", err)
	}
	if !result.Cleaned {
		t.Fatal("CleanupProjectCommand() Cleaned = false, want true")
	}
	if !strings.Contains(stderr.String(), paths.TaskBranchPrefix+"old-task") || !strings.Contains(stderr.String(), "yes") {
		t.Fatalf("cleanup prompt did not list branch and confirmation:\n%s", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(projectRoot, paths.WorktreesDirName)); !os.IsNotExist(err) {
		t.Fatalf("worktrees directory still exists or cannot be checked: %v", err)
	}
	exists, err := gitClient.BranchExists(paths.TaskBranchPrefix + "old-task")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("associated branch still exists after cleanup")
	}
}

func TestCleanupProjectCommandNoTargetsIsNoOp(t *testing.T) {
	projectRoot := t.TempDir()
	testhelpers.SetupTestGitRepo(t, projectRoot)
	var stderr bytes.Buffer

	result, err := CleanupProjectCommand(CleanupParams{ProjectRoot: projectRoot, Stderr: &stderr})
	if err != nil {
		t.Fatalf("CleanupProjectCommand() error = %v", err)
	}
	if result.Cleaned || !result.Plan.Empty() {
		t.Fatalf("CleanupProjectCommand() result = %+v, want empty no-op", result)
	}
	if stderr.Len() != 0 {
		t.Fatalf("no-op cleanup wrote stderr: %q", stderr.String())
	}
}
