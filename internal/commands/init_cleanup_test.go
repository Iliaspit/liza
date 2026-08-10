package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	gitops "github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/semble"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestInitCommandWithConfigDeclinedCleanupSkipsSemblePrewarm(t *testing.T) {
	projectRoot := setupInitCleanupProject(t)
	t.Setenv(semble.EnvEnableSemble, "true")
	runtimeMarker := filepath.Join(paths.New(projectRoot).LizaDir(), "keep")
	if err := os.MkdirAll(filepath.Dir(runtimeMarker), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runtimeMarker, []byte("keep"), 0644); err != nil {
		t.Fatal(err)
	}

	var lookups, runs int
	restore := setInitSembleHooksForTest(
		func(name string) (string, error) {
			lookups++
			return filepath.Join(projectRoot, "bin", name), nil
		},
		func(plan semble.CommandPlan) (semble.CommandResult, error) {
			runs++
			return semble.CommandResult{ExitCode: 0}, nil
		},
	)
	t.Cleanup(restore)

	err := InitCommandWithConfig(InitParams{
		Description: "Replacement goal",
		SpecRef:     "specs/vision.md",
		Stdin:       strings.NewReader("n\n"),
	})
	if err == nil || !strings.Contains(err.Error(), "initialization cancelled by user") {
		t.Fatalf("InitCommandWithConfig() error = %v, want cancellation", err)
	}
	if lookups != 0 || runs != 0 {
		t.Fatalf("declined cleanup ran Semble: lookups=%d runs=%d", lookups, runs)
	}
	if _, err := os.Stat(runtimeMarker); err != nil {
		t.Fatalf("declined cleanup removed marker: %v", err)
	}
}

func TestInitCommandWithConfigConfirmedCleanupDeletesTaskBranch(t *testing.T) {
	projectRoot := setupInitCleanupProject(t)
	lp := paths.New(projectRoot)
	if err := os.Mkdir(lp.LizaDir(), 0755); err != nil {
		t.Fatal(err)
	}
	gitClient := gitops.New(projectRoot)
	if _, err := gitClient.CreateWorktree("old-task", "HEAD"); err != nil {
		t.Fatal(err)
	}

	err := InitCommandWithConfig(InitParams{
		Description: "Replacement goal",
		SpecRef:     "specs/vision.md",
		Stdin:       strings.NewReader("y\n"),
	})
	if err != nil {
		t.Fatalf("InitCommandWithConfig() error = %v", err)
	}
	if _, err := os.Stat(lp.StatePath()); err != nil {
		t.Fatalf("replacement state file missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(projectRoot, paths.WorktreesDirName)); !os.IsNotExist(err) {
		t.Fatalf("worktrees directory still exists or cannot be checked: %v", err)
	}
	exists, err := gitClient.BranchExists(paths.TaskBranchPrefix + "old-task")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Fatal("confirmed init cleanup preserved the old task branch")
	}
}

func setupInitCleanupProject(t *testing.T) string {
	t.Helper()
	projectRoot := setupGitRepo(t)
	setupGlobalLiza(t)
	testhelpers.CreateCommittedSpecFile(t, projectRoot, "vision.md", "# Vision\n")

	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(projectRoot); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})
	return projectRoot
}
