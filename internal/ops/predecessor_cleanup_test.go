package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestCleanupPredecessorBranches_AllSuccessorsTerminal(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	// Create a branch for the predecessor (simulating a superseded task's preserved branch)
	testhelpers.MustGit(t, tmpDir, "branch", "task/predecessor-1")

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	predecessor := testhelpers.BuildTaskByStatus("predecessor-1", models.TaskStatusSuperseded, now)
	predecessor.SupersededBy = []string{"successor-1"}
	successor := testhelpers.BuildTaskByStatus("successor-1", models.TaskStatusMerged, now)
	state.Tasks = []models.Task{predecessor, successor}
	testhelpers.WriteInitialState(t, stateFile, state)

	bb := db.New(stateFile)
	gw := git.New(tmpDir)

	warnings := cleanupPredecessorBranches(bb, gw, "successor-1")
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	exists, err := gw.BranchExists("task/predecessor-1")
	if err != nil {
		t.Fatalf("BranchExists error: %v", err)
	}
	if exists {
		t.Error("predecessor branch should be deleted when all successors are terminal")
	}
}

func TestCleanupPredecessorBranches_OneSuccessorActive(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	testhelpers.MustGit(t, tmpDir, "branch", "task/predecessor-1")

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	predecessor := testhelpers.BuildTaskByStatus("predecessor-1", models.TaskStatusSuperseded, now)
	predecessor.SupersededBy = []string{"successor-1", "successor-2"}
	successor1 := testhelpers.BuildTaskByStatus("successor-1", models.TaskStatusMerged, now)
	successor2 := testhelpers.BuildTaskByStatus("successor-2", models.TaskStatusImplementing, now)
	state.Tasks = []models.Task{predecessor, successor1, successor2}
	testhelpers.WriteInitialState(t, stateFile, state)

	bb := db.New(stateFile)
	gw := git.New(tmpDir)

	warnings := cleanupPredecessorBranches(bb, gw, "successor-1")
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	exists, err := gw.BranchExists("task/predecessor-1")
	if err != nil {
		t.Fatalf("BranchExists error: %v", err)
	}
	if !exists {
		t.Error("predecessor branch should be preserved when not all successors are terminal")
	}
}

func TestCleanupPredecessorBranches_SupersededSuccessorIsTerminal(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	testhelpers.MustGit(t, tmpDir, "branch", "task/predecessor-1")

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	predecessor := testhelpers.BuildTaskByStatus("predecessor-1", models.TaskStatusSuperseded, now)
	predecessor.SupersededBy = []string{"successor-1"}
	// Successor is itself superseded — still terminal per IsTerminal()
	successor := testhelpers.BuildTaskByStatus("successor-1", models.TaskStatusSuperseded, now)
	successor.SupersededBy = []string{"successor-2"}
	state.Tasks = []models.Task{predecessor, successor}
	testhelpers.WriteInitialState(t, stateFile, state)

	bb := db.New(stateFile)
	gw := git.New(tmpDir)

	warnings := cleanupPredecessorBranches(bb, gw, "successor-1")
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	exists, err := gw.BranchExists("task/predecessor-1")
	if err != nil {
		t.Fatalf("BranchExists error: %v", err)
	}
	if exists {
		t.Error("predecessor branch should be deleted when successor is SUPERSEDED (terminal)")
	}
}

func TestCleanupPredecessorBranches_AbandonedSuccessorIsTerminal(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	testhelpers.MustGit(t, tmpDir, "branch", "task/predecessor-1")

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	predecessor := testhelpers.BuildTaskByStatus("predecessor-1", models.TaskStatusSuperseded, now)
	predecessor.SupersededBy = []string{"successor-1"}
	successor := testhelpers.BuildTaskByStatus("successor-1", models.TaskStatusAbandoned, now)
	state.Tasks = []models.Task{predecessor, successor}
	testhelpers.WriteInitialState(t, stateFile, state)

	bb := db.New(stateFile)
	gw := git.New(tmpDir)

	warnings := cleanupPredecessorBranches(bb, gw, "successor-1")
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	exists, err := gw.BranchExists("task/predecessor-1")
	if err != nil {
		t.Fatalf("BranchExists error: %v", err)
	}
	if exists {
		t.Error("predecessor branch should be deleted when successor is ABANDONED (terminal)")
	}
}

func TestCleanupPredecessorBranches_UnresolvedSuccessor(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	testhelpers.MustGit(t, tmpDir, "branch", "task/predecessor-1")

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	predecessor := testhelpers.BuildTaskByStatus("predecessor-1", models.TaskStatusSuperseded, now)
	predecessor.SupersededBy = []string{"successor-1", "ghost-task"}
	successor := testhelpers.BuildTaskByStatus("successor-1", models.TaskStatusMerged, now)
	// ghost-task is not in state
	state.Tasks = []models.Task{predecessor, successor}
	testhelpers.WriteInitialState(t, stateFile, state)

	bb := db.New(stateFile)
	gw := git.New(tmpDir)

	warnings := cleanupPredecessorBranches(bb, gw, "successor-1")

	// Should warn about unresolved successor
	hasWarning := false
	for _, w := range warnings {
		if strings.Contains(w, "ghost-task") && strings.Contains(w, "not found") {
			hasWarning = true
			break
		}
	}
	if !hasWarning {
		t.Errorf("expected warning about unresolved successor, got: %v", warnings)
	}

	// Branch should be preserved (unresolved = not terminal)
	exists, err := gw.BranchExists("task/predecessor-1")
	if err != nil {
		t.Fatalf("BranchExists error: %v", err)
	}
	if !exists {
		t.Error("predecessor branch should be preserved when a successor is unresolved")
	}
}

func TestCleanupPredecessorBranches_NoPredecessors(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now)
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	bb := db.New(stateFile)
	gw := git.New(tmpDir)

	warnings := cleanupPredecessorBranches(bb, gw, "task-1")
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

func TestCleanupPredecessorBranches_IgnoresNonSupersededWithStaleField(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	// Create a branch that should NOT be deleted
	testhelpers.MustGit(t, tmpDir, "branch", "task/stale-task")

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	// A non-SUPERSEDED task with stale SupersededBy field (should be ignored)
	staleTask := testhelpers.BuildTaskByStatus("stale-task", models.TaskStatusBlocked, now)
	staleTask.SupersededBy = []string{"successor-1"}
	successor := testhelpers.BuildTaskByStatus("successor-1", models.TaskStatusMerged, now)
	state.Tasks = []models.Task{staleTask, successor}
	testhelpers.WriteInitialState(t, stateFile, state)

	bb := db.New(stateFile)
	gw := git.New(tmpDir)

	warnings := cleanupPredecessorBranches(bb, gw, "successor-1")
	if len(warnings) > 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}

	// Branch should be preserved — task is not SUPERSEDED
	exists, err := gw.BranchExists("task/stale-task")
	if err != nil {
		t.Fatalf("BranchExists error: %v", err)
	}
	if !exists {
		t.Error("branch should not be deleted for non-SUPERSEDED task with stale SupersededBy field")
	}
}
