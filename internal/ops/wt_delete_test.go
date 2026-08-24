package ops

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestDeleteWorktree_Validation(t *testing.T) {
	t.Parallel()

	_, err := DeleteWorktree("/nonexistent", "")
	testhelpers.RequireErrorContains(t, err, "task ID is required")
}

func TestDeleteWorktree_TaskNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := DeleteWorktree(tmpDir, "nonexistent")
	if err == nil {
		t.Fatal("Expected error for nonexistent task")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestDeleteWorktree_WrongStatus(t *testing.T) {
	t.Parallel()

	statuses := []struct {
		name   string
		status models.TaskStatus
	}{
		{"IMPLEMENTING_CODE", models.TaskStatusImplementing},
		{"CODE_READY_FOR_REVIEW", models.TaskStatusReadyForReview},
		{"REVIEWING_CODE", models.TaskStatusReviewing},
		{"CODE_APPROVED", models.TaskStatusApproved},
		{"DRAFT_CODE", models.TaskStatusReady},
	}

	for _, tt := range statuses {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			statePath, _ := testhelpers.SetupLizaDir(t, tmpDir)

			now := time.Now().UTC()
			state := testhelpers.CreateValidState()
			state.Tasks = []models.Task{
				testhelpers.BuildTaskByStatus("task-1", tt.status, now),
			}
			testhelpers.WriteInitialState(t, statePath, state)

			_, err := DeleteWorktree(tmpDir, "task-1")
			testhelpers.RequireErrorContains(t, err, "cannot delete worktree")
		})
	}
}

func TestDeleteWorktree_AllowedStatuses(t *testing.T) {
	t.Parallel()

	statuses := []struct {
		name   string
		status models.TaskStatus
	}{
		{"BLOCKED", models.TaskStatusBlocked},
		{"ABANDONED", models.TaskStatusAbandoned},
		{"SUPERSEDED", models.TaskStatusSuperseded},
	}

	for _, tt := range statuses {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

			now := time.Now().UTC()
			state := testhelpers.CreateValidState()
			task := testhelpers.BuildTaskByStatus("task-1", tt.status, now)
			task.Worktree = nil // No worktree to clean up
			state.Tasks = []models.Task{task}
			testhelpers.WriteInitialState(t, stateFile, state)

			result, err := DeleteWorktree(tmpDir, "task-1")
			if err != nil {
				t.Fatalf("DeleteWorktree() error: %v", err)
			}
			if result.Existed {
				t.Error("Existed should be false when no worktree set")
			}
			if result.PreviousStatus != tt.status {
				t.Errorf("PreviousStatus = %v, want %v", result.PreviousStatus, tt.status)
			}
		})
	}
}

func TestDeleteWorktree_MergedWithWarning(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now)
	task.Worktree = nil // Already cleaned
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := DeleteWorktree(tmpDir, "task-1")
	if err != nil {
		t.Fatalf("DeleteWorktree() error: %v", err)
	}
	if result.Existed {
		t.Error("Existed should be false")
	}
}

func TestDeleteWorktree_CleanState(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	// Create a real git worktree
	gw := git.New(tmpDir)
	_, err := gw.CreateWorktree("task-1", "main")
	if err != nil {
		t.Fatalf("CreateWorktree() error: %v", err)
	}

	wtPath := filepath.Join(tmpDir, ".worktrees", "task-1")
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree directory should exist: %v", err)
	}

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatus("INTEGRATION_ANALYSIS_CLEAN"), now)
	worktree := ".worktrees/task-1"
	task.Worktree = &worktree
	task.RolePair = "integration-pair"
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := DeleteWorktree(tmpDir, "task-1")
	if err != nil {
		t.Fatalf("DeleteWorktree() error: %v", err)
	}
	if !result.Existed {
		t.Error("Existed should be true")
	}
	if result.PreviousStatus != models.TaskStatus("INTEGRATION_ANALYSIS_CLEAN") {
		t.Errorf("PreviousStatus = %v, want INTEGRATION_ANALYSIS_CLEAN", result.PreviousStatus)
	}

	// Worktree directory should be removed
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Error("worktree directory should be removed")
	}

	// Verify task.Worktree is set to nil in state
	bb := db.For(stateFile)
	updatedState, readErr := bb.Read()
	if readErr != nil {
		t.Fatalf("failed to read state: %v", readErr)
	}
	updatedTask := updatedState.FindTask("task-1")
	if updatedTask == nil {
		t.Fatal("task not found in updated state")
	}
	if updatedTask.Worktree != nil {
		t.Error("task.Worktree should be nil after deletion")
	}
}

func TestDeleteWorktree_SupersededPreservesBranch(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	// Create a real git worktree
	gw := git.New(tmpDir)
	_, err := gw.CreateWorktree("task-1", "main")
	if err != nil {
		t.Fatalf("CreateWorktree() error: %v", err)
	}

	wtPath := filepath.Join(tmpDir, ".worktrees", "task-1")
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree directory should exist: %v", err)
	}

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusSuperseded, now)
	worktree := ".worktrees/task-1"
	task.Worktree = &worktree
	task.SupersededBy = []string{"task-2"}
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := DeleteWorktree(tmpDir, "task-1")
	if err != nil {
		t.Fatalf("DeleteWorktree() error: %v", err)
	}
	if !result.Existed {
		t.Error("Existed should be true")
	}
	if len(result.Warnings) > 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}

	// Worktree directory should be removed
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Error("worktree directory should be removed")
	}

	// Branch should be preserved for successors
	exists, brErr := gw.BranchExists("task/task-1")
	if brErr != nil {
		t.Fatalf("BranchExists error: %v", brErr)
	}
	if !exists {
		t.Error("branch should be preserved for SUPERSEDED task — successors may need it")
	}
}
