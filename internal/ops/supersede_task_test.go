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

func TestSupersedeTask_Validation(t *testing.T) {
	tests := []struct {
		name           string
		taskID         string
		replacementIDs []string
		reason         string
		wantErr        string
	}{
		{
			name: "empty task ID", replacementIDs: []string{"r1"}, reason: "r",
			wantErr: "task ID is required",
		},
		{
			name: "no replacements and no reason", taskID: "t1", replacementIDs: []string{},
			wantErr: "rescope reason is required",
		},
		{
			name: "empty reason", taskID: "t1", replacementIDs: []string{"r1"},
			wantErr: "rescope reason is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SupersedeTask("/nonexistent", tt.taskID, tt.replacementIDs, tt.reason, "orchestrator-1")
			testhelpers.RequireErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestSupersedeTask_FromBlocked(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := SupersedeTask(tmpDir, "task-1", []string{"task-2", "task-3"}, "Split into smaller tasks", "orchestrator-1")
	if err != nil {
		t.Fatalf("SupersedeTask() error: %v", err)
	}

	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}
	if result.OriginalStatus != models.TaskStatusBlocked {
		t.Errorf("OriginalStatus = %v, want BLOCKED", result.OriginalStatus)
	}
	if len(result.ReplacementIDs) != 2 {
		t.Errorf("ReplacementIDs len = %d, want 2", len(result.ReplacementIDs))
	}

	// Worktree cleanup is best-effort — directory doesn't exist so no warnings
	// (RemoveWorktreeDir gracefully handles missing directories)

	// Verify state
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := readState.FindTask("task-1")
	if task == nil {
		t.Fatal("Task not found")
	}
	if task.Status != models.TaskStatusSuperseded {
		t.Errorf("Status = %v, want SUPERSEDED", task.Status)
	}
	if len(task.SupersededBy) != 2 || task.SupersededBy[0] != "task-2" {
		t.Errorf("SupersededBy = %v, want [task-2 task-3]", task.SupersededBy)
	}
	if task.RescopeReason == nil || *task.RescopeReason != "Split into smaller tasks" {
		t.Error("RescopeReason not set correctly")
	}
	if task.AssignedTo != nil {
		t.Error("AssignedTo should be nil after superseding")
	}
	if task.Worktree != nil {
		t.Error("Worktree should be nil after superseding")
	}

	lastHistory := task.History[len(task.History)-1]
	if lastHistory.Event != models.TaskEventSuperseded {
		t.Errorf("History event = %q, want %q", lastHistory.Event, models.TaskEventSuperseded)
	}
}

func TestSupersedeTask_NoReplacements(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := SupersedeTask(tmpDir, "task-1", nil, "Work already merged in prior sprint", "orchestrator-1")
	if err != nil {
		t.Fatalf("SupersedeTask() error: %v", err)
	}

	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}
	if len(result.ReplacementIDs) != 0 {
		t.Errorf("ReplacementIDs = %v, want empty", result.ReplacementIDs)
	}

	// Verify state
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := readState.FindTask("task-1")
	if task == nil {
		t.Fatal("Task not found")
	}
	if task.Status != models.TaskStatusSuperseded {
		t.Errorf("Status = %v, want SUPERSEDED", task.Status)
	}
	if len(task.SupersededBy) != 0 {
		t.Errorf("SupersededBy = %v, want empty", task.SupersededBy)
	}
	if task.RescopeReason == nil || *task.RescopeReason != "Work already merged in prior sprint" {
		t.Error("RescopeReason not set correctly")
	}

	lastHistory := task.History[len(task.History)-1]
	if lastHistory.Note == nil || *lastHistory.Note != "superseded without replacements" {
		t.Errorf("History note = %v, want 'superseded without replacements'", lastHistory.Note)
	}
}

func TestSupersedeTask_RejectsDownstreamReplacement(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	planningTask := testhelpers.BuildTaskByStatus("plan-1", models.TaskStatusBlocked, now)
	planningTask.RolePair = "code-planning-pair"
	codingTask := testhelpers.BuildTaskByStatus("coding-1", models.TaskStatusMerged, now)
	codingTask.RolePair = "coding-pair"

	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{planningTask, codingTask}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SupersedeTask(tmpDir, "plan-1", []string{"coding-1"}, "Coding work already exists", "orchestrator-1")
	testhelpers.RequireErrorContains(t, err, "role_pair coding-pair is downstream of code-planning-pair")
}

func TestSupersedeTask_NoReplacements_DeletesBranch(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	// Create a branch for the task
	gw := git.New(tmpDir)
	_, err := gw.CreateWorktree("task-1", "main")
	if err != nil {
		t.Fatalf("CreateWorktree() error: %v", err)
	}

	// Verify branch exists
	exists, err := gw.BranchExists("task/task-1")
	if err != nil {
		t.Fatalf("BranchExists error: %v", err)
	}
	if !exists {
		t.Fatal("branch should exist before supersede")
	}

	// Set up state with BLOCKED task that has a worktree
	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
	worktree := ".worktrees/task-1"
	task.Worktree = &worktree
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := SupersedeTask(tmpDir, "task-1", nil, "Completed externally", "orchestrator-1")
	if err != nil {
		t.Fatalf("SupersedeTask() error: %v", err)
	}

	// No warnings expected
	if len(result.Warnings) > 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}

	// Verify worktree directory removed
	wtPath := filepath.Join(tmpDir, ".worktrees", "task-1")
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Error("worktree directory should be removed")
	}

	// Verify branch deleted — no successors to preserve it for
	exists, err = gw.BranchExists("task/task-1")
	if err != nil {
		t.Fatalf("BranchExists error: %v", err)
	}
	if exists {
		t.Error("branch should be deleted when superseding with no replacements")
	}
}

func TestSupersedeTask_FromRejected(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusRejected, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := SupersedeTask(tmpDir, "task-1", []string{"task-2"}, "Rewrite needed", "orchestrator-1")
	if err != nil {
		t.Fatalf("SupersedeTask() error: %v", err)
	}
	if result.OriginalStatus != models.TaskStatusRejected {
		t.Errorf("OriginalStatus = %v, want REJECTED", result.OriginalStatus)
	}
}

func TestSupersedeTask_FromIntegrationFailedMissingWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusIntegrationFailed, now)
	worktree := ".worktrees/task-1"
	task.Worktree = &worktree
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := SupersedeTask(tmpDir, "task-1", nil, "Completed externally by merged PR", "orchestrator-1")
	if err != nil {
		t.Fatalf("SupersedeTask() error: %v", err)
	}
	if result.OriginalStatus != models.TaskStatusIntegrationFailed {
		t.Errorf("OriginalStatus = %v, want INTEGRATION_FAILED", result.OriginalStatus)
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	updated := readState.FindTask("task-1")
	if updated == nil {
		t.Fatal("Task not found")
	}
	if updated.Status != models.TaskStatusSuperseded {
		t.Errorf("Status = %v, want SUPERSEDED", updated.Status)
	}
	if updated.Worktree != nil {
		t.Errorf("Worktree = %v, want nil", *updated.Worktree)
	}
}

func TestSupersedeTask_FromReady(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SupersedeTask(tmpDir, "task-1", []string{"task-2"}, "No longer needed", "orchestrator-1")
	if err != nil {
		t.Fatalf("SupersedeTask() error: %v", err)
	}
}

func TestSupersedeTask_FromPipelineDraftState(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	// DRAFT_ARCHITECTURE is a pipeline-declared initial state, not hardcoded.
	task := models.Task{
		ID:          "task-1",
		Type:        models.TaskTypeCoding,
		Description: "Architecture task",
		Status:      "DRAFT_ARCHITECTURE",
		Priority:    1,
		Created:     now,
		SpecRef:     "README.md",
		DoneWhen:    "Architecture defined",
		Scope:       "Test scope",
		History:     []models.TaskHistoryEntry{},
		RolePair:    "architecture-pair",
	}
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := SupersedeTask(tmpDir, "task-1", []string{"task-2"}, "Scope changed", "orchestrator-1")
	if err != nil {
		t.Fatalf("SupersedeTask() error: %v", err)
	}
	if result.OriginalStatus != "DRAFT_ARCHITECTURE" {
		t.Errorf("OriginalStatus = %v, want DRAFT_ARCHITECTURE", result.OriginalStatus)
	}
}

func TestSupersedeTask_LegacyTaskNoRolePair(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	// Legacy task with no role_pair — must still be supersedeable from DRAFT_CODE.
	task := models.Task{
		ID:          "task-1",
		Type:        models.TaskTypeCoding,
		Description: "Legacy task",
		Status:      models.TaskStatusReady, // DRAFT_CODE
		Priority:    1,
		Created:     now,
		SpecRef:     "README.md",
		DoneWhen:    "Done",
		Scope:       "Test scope",
		History:     []models.TaskHistoryEntry{},
		RolePair:    "", // no role-pair — legacy
	}
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := SupersedeTask(tmpDir, "task-1", []string{"task-2"}, "Legacy rescope", "orchestrator-1")
	if err != nil {
		t.Fatalf("SupersedeTask() error: %v", err)
	}
	if result.OriginalStatus != models.TaskStatusReady {
		t.Errorf("OriginalStatus = %v, want DRAFT_CODE", result.OriginalStatus)
	}
}

func TestSupersedeTask_WrongStatus(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SupersedeTask(tmpDir, "task-1", []string{"task-2"}, "reason", "orchestrator-1")
	testhelpers.RequireErrorContains(t, err, "cannot supersede")
}

func TestSupersedeTask_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := SupersedeTask(tmpDir, "nonexistent", []string{"task-2"}, "reason", "orchestrator-1")
	if err == nil {
		t.Fatal("Expected error for nonexistent task")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestSupersedeTask_EmptyAgentIDReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	// Empty agentID should now return an error
	_, err := SupersedeTask(tmpDir, "task-1", []string{"task-2"}, "reason", "")
	if err == nil {
		t.Fatal("expected error for empty agentID, got nil")
	}
	testhelpers.AssertErrorContains(t, err, "orchestrator agent ID is required")
}

func TestSupersedeTask_CleansUpWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	// Create a real git worktree
	gw := git.New(tmpDir)
	_, err := gw.CreateWorktree("task-1", "main")
	if err != nil {
		t.Fatalf("CreateWorktree() error: %v", err)
	}

	// Verify worktree directory exists
	wtPath := filepath.Join(tmpDir, ".worktrees", "task-1")
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree directory should exist: %v", err)
	}

	// Set up state with BLOCKED task that has a worktree
	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
	worktree := ".worktrees/task-1"
	task.Worktree = &worktree
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := SupersedeTask(tmpDir, "task-1", []string{"task-2"}, "Split into smaller tasks", "orchestrator-1")
	if err != nil {
		t.Fatalf("SupersedeTask() error: %v", err)
	}

	// No warnings expected — cleanup should succeed with real git repo
	if len(result.Warnings) > 0 {
		t.Errorf("unexpected warnings: %v", result.Warnings)
	}

	// Verify worktree directory removed
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Error("worktree directory should be removed after supersede")
	}

	// Verify state: Worktree field cleared
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}
	updatedTask := readState.FindTask("task-1")
	if updatedTask.Worktree != nil {
		t.Error("Worktree should be nil in state after supersede")
	}

	// Verify branch preserved — successors may need it via git show
	exists, brErr := gw.BranchExists("task/task-1")
	if brErr != nil {
		t.Fatalf("BranchExists error: %v", brErr)
	}
	if !exists {
		t.Error("task branch should be preserved after supersede for successor access")
	}
}
