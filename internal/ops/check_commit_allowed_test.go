package ops

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestCheckCommitAllowed_EmptyTaskID(t *testing.T) {
	t.Parallel()

	result := CheckCommitAllowed(t.TempDir(), "")
	if !result.Allowed {
		t.Fatalf("empty task ID should fail-safe allow, got reject: %s", result.Reason)
	}
}

func TestCheckCommitAllowed_StateMissing(t *testing.T) {
	t.Parallel()

	// Directly exercises the spec risk "hook must handle missing state gracefully":
	// No project state on disk must result in fail-safe allow.
	tmpDir := t.TempDir()
	result := CheckCommitAllowed(tmpDir, "task-1")
	if !result.Allowed {
		t.Fatalf("missing state should fail-safe allow, got reject: %s", result.Reason)
	}
}

func TestCheckCommitAllowed_TaskNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	result := CheckCommitAllowed(tmpDir, "nonexistent")
	if !result.Allowed {
		t.Fatalf("missing task should fail-safe allow, got reject: %s", result.Reason)
	}
}

func TestCheckCommitAllowed_Blocked(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)}
	testhelpers.WriteInitialState(t, stateFile, state)

	result := CheckCommitAllowed(tmpDir, "task-1")
	if !result.Allowed {
		t.Fatalf("BLOCKED task should allow commit, got reject: %s", result.Reason)
	}
}

func TestCheckCommitAllowed_Implementing(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now)}
	testhelpers.WriteInitialState(t, stateFile, state)

	result := CheckCommitAllowed(tmpDir, "task-1")
	if !result.Allowed {
		t.Fatalf("IMPLEMENTING task should allow commit, got reject: %s", result.Reason)
	}
}

func TestCheckCommitAllowed_Rejected(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now)}
	testhelpers.WriteInitialState(t, stateFile, state)

	result := CheckCommitAllowed(tmpDir, "task-1")
	if !result.Allowed {
		t.Fatalf("REJECTED task should allow commit (coder addresses feedback), got reject: %s", result.Reason)
	}
}

func TestCheckCommitAllowed_ReadyForReview(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, now)}
	testhelpers.WriteInitialState(t, stateFile, state)

	result := CheckCommitAllowed(tmpDir, "task-1")
	if result.Allowed {
		t.Fatalf("READY_FOR_REVIEW must reject commits — worktree HEAD must not advance past review_commit")
	}
	if result.Reason == "" {
		t.Error("rejection must include a reason for the user")
	}
}

func TestCheckCommitAllowed_Reviewing(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)}
	testhelpers.WriteInitialState(t, stateFile, state)

	result := CheckCommitAllowed(tmpDir, "task-1")
	if result.Allowed {
		t.Fatalf("REVIEWING must reject commits — this is the exact failure mode the spec targets")
	}
}

func TestCheckCommitAllowed_Approved(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{testhelpers.BuildTaskByStatus("task-1", models.TaskStatusApproved, now)}
	testhelpers.WriteInitialState(t, stateFile, state)

	result := CheckCommitAllowed(tmpDir, "task-1")
	if result.Allowed {
		t.Fatalf("APPROVED must reject commits — awaiting merge, no further mutations")
	}
}

func TestCheckCommitAllowed_MissingRolePair(t *testing.T) {
	t.Parallel()

	// Regression guard: a task with no role_pair must fail-safe allow, not
	// reject. Rejecting would turn state-schema gaps into commit deadlocks.
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	task.RolePair = "" // simulate schema gap / legacy record
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	result := CheckCommitAllowed(tmpDir, "task-1")
	if !result.Allowed {
		t.Fatalf("missing role_pair must fail-safe allow, got reject: %s", result.Reason)
	}
}

func TestCheckCommitAllowed_UnknownRolePair(t *testing.T) {
	t.Parallel()

	// Regression guard: a task whose role_pair isn't in the pipeline config
	// (e.g. config drift) must fail-safe allow. Pipeline resolver errors
	// during RejectedStatus lookup must not become commit deadlocks.
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)
	task.RolePair = "nonexistent-role-pair"
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	result := CheckCommitAllowed(tmpDir, "task-1")
	if !result.Allowed {
		t.Fatalf("unknown role_pair must fail-safe allow (pipeline drift), got reject: %s", result.Reason)
	}
}

func TestCheckCommitAllowed_PipelineMissing(t *testing.T) {
	t.Parallel()

	// Fail-safe when pipeline config is unavailable.
	tmpDir := t.TempDir()
	_, _ = testhelpers.SetupLizaDir(t, tmpDir)

	// Remove pipeline.yaml that SetupLizaDir installed.
	_ = os.Remove(filepath.Join(tmpDir, paths.ProjectDirName(), "pipeline.yaml"))

	stateFile := filepath.Join(tmpDir, paths.ProjectDirName(), "state.yaml")
	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, now)}
	testhelpers.WriteInitialState(t, stateFile, state)

	result := CheckCommitAllowed(tmpDir, "task-1")
	if !result.Allowed {
		t.Fatalf("missing pipeline config should fail-safe allow, got reject: %s", result.Reason)
	}
}
