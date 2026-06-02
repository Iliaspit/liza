package ops

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestUnblockTask_RestoresExecutingStateAndAssignment(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Config.LeaseDuration = 1800
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
	task.RolePair = "code-planning-pair"
	task.Worktree = testhelpers.StringPtr(".worktrees/task-1")
	task.RepairRequest = &models.RepairRequest{
		Operation:  "restore_git_write_access",
		Target:     ".git/worktrees/task-1",
		Command:    "git -C .worktrees/task-1 add plan.md",
		Evidence:   []string{"command=git -C .worktrees/task-1 add plan.md exit_code=128 stderr=fatal: Unable to create index.lock: Read-only file system"},
		Validation: []string{"git -C .worktrees/task-1 add plan.md"},
	}
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	bb := db.New(stateFile)
	testhelpers.RegisterTestAgent(t, bb, "code-planner-1", "code-planner")
	setAgentPID(t, bb, "code-planner-1", os.Getpid())

	result, err := UnblockTask(tmpDir, "task-1", "code-planner-1", "git metadata repair verified", "orchestrator-1")
	if err != nil {
		t.Fatalf("UnblockTask() error: %v", err)
	}
	if result.ToStatus != models.TaskStatusCodePlanning {
		t.Fatalf("ToStatus = %s, want %s", result.ToStatus, models.TaskStatusCodePlanning)
	}
	if result.AssignedTo != "code-planner-1" {
		t.Fatalf("AssignedTo = %q, want code-planner-1", result.AssignedTo)
	}

	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Read state: %v", err)
	}
	readTask := readState.FindTask("task-1")
	if readTask == nil {
		t.Fatal("task not found")
	}
	if readTask.Status != models.TaskStatusCodePlanning {
		t.Errorf("Status = %s, want %s", readTask.Status, models.TaskStatusCodePlanning)
	}
	if readTask.AssignedTo == nil || *readTask.AssignedTo != "code-planner-1" {
		t.Fatalf("AssignedTo = %v, want code-planner-1", readTask.AssignedTo)
	}
	if readTask.LeaseExpires == nil {
		t.Fatal("LeaseExpires is nil")
	}
	if readTask.BlockedReason != nil {
		t.Fatal("BlockedReason should be cleared")
	}
	if len(readTask.BlockedQuestions) != 0 {
		t.Fatalf("BlockedQuestions len = %d, want 0", len(readTask.BlockedQuestions))
	}
	if readTask.RepairRequest != nil {
		t.Fatal("RepairRequest should be cleared")
	}
	last := readTask.History[len(readTask.History)-1]
	if last.Event != models.TaskEventUnblocked {
		t.Fatalf("History event = %q, want %q", last.Event, models.TaskEventUnblocked)
	}
	validation, ok := last.Extra["repair_validation"].([]any)
	if !ok {
		t.Fatalf("repair_validation history extra = %T, want []any", last.Extra["repair_validation"])
	}
	if len(validation) != 1 || validation[0] != "git -C .worktrees/task-1 add plan.md" {
		t.Fatalf("repair_validation = %v", validation)
	}

	agent := readState.Agents["code-planner-1"]
	if agent.Status != models.AgentStatusWorking {
		t.Fatalf("Agent status = %s, want %s", agent.Status, models.AgentStatusWorking)
	}
	if agent.CurrentTask == nil || *agent.CurrentTask != "task-1" {
		t.Fatalf("Agent current task = %v, want task-1", agent.CurrentTask)
	}
}

func TestUnblockTask_AllowsAssignToWithoutLiveProcess(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
	task.RolePair = "code-planning-pair"
	task.Worktree = testhelpers.StringPtr(".worktrees/task-1")
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	bb := db.New(stateFile)
	testhelpers.RegisterTestAgent(t, bb, "code-planner-1", "code-planner")
	if err := bb.Modify(func(s *models.State) error {
		agent := s.Agents["code-planner-1"]
		agent.PID = -1
		s.Agents["code-planner-1"] = agent
		return nil
	}); err != nil {
		t.Fatalf("Failed to corrupt agent PID: %v", err)
	}

	result, err := UnblockTask(tmpDir, "task-1", "code-planner-1", "repair verified", "orchestrator-1")
	if err != nil {
		t.Fatalf("UnblockTask() error: %v", err)
	}
	if result.AssignedTo != "code-planner-1" {
		t.Fatalf("AssignedTo = %q, want code-planner-1", result.AssignedTo)
	}
}

func TestUnblockTask_WithoutAssignToMakesTaskClaimable(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
	task.RolePair = "code-planning-pair"
	task.Worktree = nil
	task.BaseCommit = nil
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := UnblockTaskWithOptions(tmpDir, "task-1", "repair verified", "orchestrator-1", UnblockTaskOptions{})
	if err != nil {
		t.Fatalf("UnblockTaskWithOptions() error: %v", err)
	}
	if !result.Claimable {
		t.Fatal("Claimable = false, want true")
	}
	if result.LeaseExpires != nil {
		t.Fatalf("LeaseExpires = %v, want nil", result.LeaseExpires)
	}
	if result.ToStatus != models.TaskStatusDraftCodingPlan {
		t.Fatalf("ToStatus = %s, want %s", result.ToStatus, models.TaskStatusDraftCodingPlan)
	}

	readState, err := db.New(stateFile).Read()
	if err != nil {
		t.Fatalf("Read state: %v", err)
	}
	updated := readState.FindTask("task-1")
	if updated == nil {
		t.Fatal("task not found")
	}
	if updated.Status != models.TaskStatusDraftCodingPlan {
		t.Fatalf("Status = %s, want %s", updated.Status, models.TaskStatusDraftCodingPlan)
	}
	if updated.AssignedTo != nil {
		t.Fatalf("AssignedTo = %v, want nil", *updated.AssignedTo)
	}
	if updated.Worktree != nil {
		t.Fatalf("Worktree = %v, want nil", *updated.Worktree)
	}
	if updated.BlockedReason != nil || len(updated.BlockedQuestions) != 0 || updated.RepairRequest != nil {
		t.Fatalf("blocked metadata not cleared: %+v", updated)
	}
}

func TestClaimTask_PreservesUnblockedWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.CreateTestWorktree(t, tmpDir, "task-1")

	baseCommit := testhelpers.MustGit(t, tmpDir, "rev-parse", "integration")
	wtDir := filepath.Join(tmpDir, ".worktrees", "task-1")
	planPath := filepath.Join(wtDir, "plan.md")
	if err := os.WriteFile(planPath, []byte("preserved plan\n"), 0644); err != nil {
		t.Fatalf("write plan: %v", err)
	}
	testhelpers.MustGit(t, wtDir, "add", "plan.md")
	testhelpers.MustGit(t, wtDir, "commit", "-m", "Preserved plan")
	preservedHead := testhelpers.MustGit(t, wtDir, "rev-parse", "HEAD")

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
	task.RolePair = "code-planning-pair"
	task.BaseCommit = &baseCommit
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	if _, err := UnblockTaskWithOptions(tmpDir, "task-1", "repair verified", "orchestrator-1", UnblockTaskOptions{}); err != nil {
		t.Fatalf("UnblockTaskWithOptions() error: %v", err)
	}

	bb := db.New(stateFile)
	testhelpers.RegisterTestAgent(t, bb, "code-planner-1", "code-planner")
	setAgentPID(t, bb, "code-planner-1", os.Getpid())
	result, err := ClaimTask(tmpDir, "task-1", "code-planner-1")
	if err != nil {
		t.Fatalf("ClaimTask() error: %v", err)
	}
	if result.WorktreeRecreated {
		t.Fatal("WorktreeRecreated = true, want false")
	}
	claimedHead := testhelpers.MustGit(t, wtDir, "rev-parse", "HEAD")
	if claimedHead != preservedHead {
		t.Fatalf("HEAD = %s, want preserved %s", claimedHead, preservedHead)
	}
	if _, err := os.Stat(planPath); err != nil {
		t.Fatalf("preserved file missing after claim: %v", err)
	}
}

func TestUnblockTask_RebaseOnMakesClaimableAndUpdatesBaseCommit(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.CreateTestWorktree(t, tmpDir, "task-1")

	baseCommit := testhelpers.MustGit(t, tmpDir, "rev-parse", "integration")
	wtDir := filepath.Join(tmpDir, ".worktrees", "task-1")
	writeAndCommit(t, wtDir, "task.txt", "task work\n", "Task work")
	oldHead := testhelpers.MustGit(t, wtDir, "rev-parse", "HEAD")
	writeAndCommit(t, tmpDir, "integration.txt", "integration move\n", "Move integration")
	targetSHA := testhelpers.MustGit(t, tmpDir, "rev-parse", "HEAD")
	testhelpers.MustGit(t, tmpDir, "branch", "-f", "integration", targetSHA)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
	task.RolePair = "code-planning-pair"
	task.BaseCommit = &baseCommit
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := UnblockTaskWithOptions(tmpDir, "task-1", "repair verified", "orchestrator-1", UnblockTaskOptions{RebaseOn: "integration"})
	if err != nil {
		t.Fatalf("UnblockTaskWithOptions() error: %v", err)
	}
	if result.Rebase == nil {
		t.Fatal("Rebase result is nil")
	}
	if result.Rebase.OldHead != oldHead {
		t.Fatalf("OldHead = %s, want %s", result.Rebase.OldHead, oldHead)
	}
	if result.Rebase.TargetSHA != targetSHA {
		t.Fatalf("TargetSHA = %s, want %s", result.Rebase.TargetSHA, targetSHA)
	}

	readState, err := db.New(stateFile).Read()
	if err != nil {
		t.Fatalf("Read state: %v", err)
	}
	updated := readState.FindTask("task-1")
	if updated == nil {
		t.Fatal("task not found")
	}
	if updated.Status != models.TaskStatusDraftCodingPlan {
		t.Fatalf("Status = %s, want %s", updated.Status, models.TaskStatusDraftCodingPlan)
	}
	if updated.BaseCommit == nil || *updated.BaseCommit != targetSHA {
		t.Fatalf("BaseCommit = %v, want %s", updated.BaseCommit, targetSHA)
	}
	if result.Rebase.NewHead != testhelpers.MustGit(t, wtDir, "rev-parse", "HEAD") {
		t.Fatalf("NewHead does not match worktree HEAD")
	}
}

func TestUnblockTask_RebaseOnAssignToResumesAndUpdatesBaseCommit(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.CreateTestWorktree(t, tmpDir, "task-1")

	baseCommit := testhelpers.MustGit(t, tmpDir, "rev-parse", "integration")
	wtDir := filepath.Join(tmpDir, ".worktrees", "task-1")
	writeAndCommit(t, wtDir, "task.txt", "task work\n", "Task work")
	oldHead := testhelpers.MustGit(t, wtDir, "rev-parse", "HEAD")
	writeAndCommit(t, tmpDir, "integration.txt", "integration move\n", "Move integration")
	targetSHA := testhelpers.MustGit(t, tmpDir, "rev-parse", "HEAD")
	testhelpers.MustGit(t, tmpDir, "branch", "-f", "integration", targetSHA)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
	task.RolePair = "code-planning-pair"
	task.BaseCommit = &baseCommit
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	bb := db.New(stateFile)
	testhelpers.RegisterTestAgent(t, bb, "code-planner-1", "code-planner")
	if err := bb.Modify(func(s *models.State) error {
		agent := s.Agents["code-planner-1"]
		agent.PID = -1
		s.Agents["code-planner-1"] = agent
		return nil
	}); err != nil {
		t.Fatalf("set dead agent PID: %v", err)
	}

	result, err := UnblockTaskWithOptions(tmpDir, "task-1", "repair verified", "orchestrator-1", UnblockTaskOptions{
		AssignTo: "code-planner-1",
		RebaseOn: "integration",
	})
	if err != nil {
		t.Fatalf("UnblockTaskWithOptions() error: %v", err)
	}
	if result.Claimable {
		t.Fatal("Claimable = true, want false")
	}
	if result.ToStatus != models.TaskStatusCodePlanning {
		t.Fatalf("ToStatus = %s, want %s", result.ToStatus, models.TaskStatusCodePlanning)
	}
	if result.AssignedTo != "code-planner-1" {
		t.Fatalf("AssignedTo = %q, want code-planner-1", result.AssignedTo)
	}
	if result.LeaseExpires == nil {
		t.Fatal("LeaseExpires is nil, want direct-resume lease")
	}
	if result.Rebase == nil {
		t.Fatal("Rebase result is nil")
	}
	if result.Rebase.OldHead != oldHead {
		t.Fatalf("OldHead = %s, want %s", result.Rebase.OldHead, oldHead)
	}
	if result.Rebase.TargetSHA != targetSHA {
		t.Fatalf("TargetSHA = %s, want %s", result.Rebase.TargetSHA, targetSHA)
	}

	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Read state: %v", err)
	}
	updated := readState.FindTask("task-1")
	if updated == nil {
		t.Fatal("task not found")
	}
	if updated.Status != models.TaskStatusCodePlanning {
		t.Fatalf("Status = %s, want %s", updated.Status, models.TaskStatusCodePlanning)
	}
	if updated.BaseCommit == nil || *updated.BaseCommit != targetSHA {
		t.Fatalf("BaseCommit = %v, want %s", updated.BaseCommit, targetSHA)
	}
	agent := readState.Agents["code-planner-1"]
	if agent.CurrentTask == nil || *agent.CurrentTask != "task-1" {
		t.Fatalf("Agent current task = %v, want task-1", agent.CurrentTask)
	}
}

func TestUnblockTask_RebaseOnRejectsTrackedDirtyWithoutAllowDirty(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.CreateTestWorktree(t, tmpDir, "task-1")

	baseCommit := testhelpers.MustGit(t, tmpDir, "rev-parse", "integration")
	wtDir := filepath.Join(tmpDir, ".worktrees", "task-1")
	writeAndCommit(t, wtDir, "task.txt", "task work\n", "Task work")
	if err := os.WriteFile(filepath.Join(wtDir, "task.txt"), []byte("dirty task work\n"), 0644); err != nil {
		t.Fatalf("dirty write: %v", err)
	}
	writeAndCommit(t, tmpDir, "integration.txt", "integration move\n", "Move integration")
	targetSHA := testhelpers.MustGit(t, tmpDir, "rev-parse", "HEAD")
	testhelpers.MustGit(t, tmpDir, "branch", "-f", "integration", targetSHA)

	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, time.Now().UTC())
	task.RolePair = "code-planning-pair"
	task.BaseCommit = &baseCommit
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := UnblockTaskWithOptions(tmpDir, "task-1", "repair verified", "orchestrator-1", UnblockTaskOptions{RebaseOn: "integration"})
	if err == nil {
		t.Fatal("expected dirty worktree error, got nil")
	}
	if !strings.Contains(err.Error(), "--allow-dirty") {
		t.Fatalf("error = %q, want --allow-dirty hint", err.Error())
	}
}

func TestUnblockTask_RebaseOnRejectsUntrackedOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.CreateTestWorktree(t, tmpDir, "task-1")

	baseCommit := testhelpers.MustGit(t, tmpDir, "rev-parse", "integration")
	wtDir := filepath.Join(tmpDir, ".worktrees", "task-1")
	if err := os.WriteFile(filepath.Join(wtDir, "future.txt"), []byte("local untracked\n"), 0644); err != nil {
		t.Fatalf("write untracked: %v", err)
	}
	writeAndCommit(t, tmpDir, "future.txt", "integration file\n", "Add future file")
	targetSHA := testhelpers.MustGit(t, tmpDir, "rev-parse", "HEAD")
	testhelpers.MustGit(t, tmpDir, "branch", "-f", "integration", targetSHA)

	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, time.Now().UTC())
	task.RolePair = "code-planning-pair"
	task.BaseCommit = &baseCommit
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := UnblockTaskWithOptions(tmpDir, "task-1", "repair verified", "orchestrator-1", UnblockTaskOptions{RebaseOn: "integration"})
	if err == nil {
		t.Fatal("expected untracked overwrite error, got nil")
	}
	if !strings.Contains(err.Error(), "untracked files") || !strings.Contains(err.Error(), "future.txt") {
		t.Fatalf("error = %q, want untracked future.txt", err.Error())
	}
}

func TestUnblockTask_RebaseConflictLeavesBlockedWithRepairRequest(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	writeAndCommit(t, tmpDir, "conflict.txt", "base\n", "Add base conflict file")
	testhelpers.MustGit(t, tmpDir, "branch", "-f", "integration", "HEAD")
	testhelpers.CreateTestWorktree(t, tmpDir, "task-1")

	baseCommit := testhelpers.MustGit(t, tmpDir, "rev-parse", "integration")
	wtDir := filepath.Join(tmpDir, ".worktrees", "task-1")
	writeAndCommit(t, wtDir, "conflict.txt", "task\n", "Task conflict edit")
	writeAndCommit(t, tmpDir, "conflict.txt", "integration\n", "Integration conflict edit")
	targetSHA := testhelpers.MustGit(t, tmpDir, "rev-parse", "HEAD")
	testhelpers.MustGit(t, tmpDir, "branch", "-f", "integration", targetSHA)

	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, time.Now().UTC())
	task.RolePair = "code-planning-pair"
	task.BaseCommit = &baseCommit
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := UnblockTaskWithOptions(tmpDir, "task-1", "repair verified", "orchestrator-1", UnblockTaskOptions{RebaseOn: "integration"})
	if err == nil {
		t.Fatal("expected rebase conflict error, got nil")
	}
	var unblockConflict *UnblockRebaseConflictError
	if !stderrors.As(err, &unblockConflict) {
		t.Fatalf("error = %T %v, want UnblockRebaseConflictError", err, err)
	}

	readState, readErr := db.New(stateFile).Read()
	if readErr != nil {
		t.Fatalf("Read state: %v", readErr)
	}
	updated := readState.FindTask("task-1")
	if updated == nil {
		t.Fatal("task not found")
	}
	if updated.Status != models.TaskStatusBlocked {
		t.Fatalf("Status = %s, want BLOCKED", updated.Status)
	}
	if updated.BlockedReason == nil || !strings.Contains(*updated.BlockedReason, "rebase conflict") {
		t.Fatalf("BlockedReason = %v, want rebase conflict", updated.BlockedReason)
	}
	if updated.RepairRequest == nil {
		t.Fatal("RepairRequest is nil")
	}
	if updated.RepairRequest.Operation != "resolve_unblock_rebase_conflict" {
		t.Fatalf("RepairRequest.Operation = %q", updated.RepairRequest.Operation)
	}
	if updated.IntegrationFailure != nil {
		t.Fatalf("IntegrationFailure = %v, want nil", updated.IntegrationFailure)
	}
	last := updated.History[len(updated.History)-1]
	if last.Event != models.TaskEventBlocked {
		t.Fatalf("last event = %s, want blocked", last.Event)
	}
	if status := testhelpers.MustGit(t, wtDir, "status", "--short"); strings.Contains(status, "UU ") {
		t.Fatalf("rebase was not aborted; status = %q", status)
	}
}

func TestUnblockTask_RejectsUnmetDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
	task.RolePair = "code-planning-pair"
	task.Worktree = testhelpers.StringPtr(".worktrees/task-1")
	task.DependsOn = []string{"dep-1"}
	dep := testhelpers.BuildTaskByStatus("dep-1", models.TaskStatusImplementing, now)
	state.Tasks = []models.Task{task, dep}
	testhelpers.WriteInitialState(t, stateFile, state)

	bb := db.New(stateFile)
	testhelpers.RegisterTestAgent(t, bb, "code-planner-1", "code-planner")
	setAgentPID(t, bb, "code-planner-1", os.Getpid())

	_, err := UnblockTask(tmpDir, "task-1", "code-planner-1", "repair verified", "orchestrator-1")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "unmet dependencies: dep-1") {
		t.Fatalf("Error = %q, want unmet dependencies", err.Error())
	}
}

func TestUnblockTask_AllowsMergedDependency(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
	task.RolePair = "code-planning-pair"
	task.Worktree = testhelpers.StringPtr(".worktrees/task-1")
	task.DependsOn = []string{"dep-1"}
	dep := testhelpers.BuildTaskByStatus("dep-1", models.TaskStatusMerged, now)
	state.Tasks = []models.Task{task, dep}
	testhelpers.WriteInitialState(t, stateFile, state)

	bb := db.New(stateFile)
	testhelpers.RegisterTestAgent(t, bb, "code-planner-1", "code-planner")
	setAgentPID(t, bb, "code-planner-1", os.Getpid())

	_, err := UnblockTask(tmpDir, "task-1", "code-planner-1", "repair verified", "orchestrator-1")
	if err != nil {
		t.Fatalf("UnblockTask() error: %v", err)
	}
}

func TestUnblockTask_RejectsUnknownAssignTo(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
	task.RolePair = "code-planning-pair"
	task.Worktree = testhelpers.StringPtr(".worktrees/task-1")
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := UnblockTask(tmpDir, "task-1", "code-planner-99", "repair verified", "orchestrator-1")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "is not registered") {
		t.Fatalf("Error = %q, want not registered", err.Error())
	}
}

func TestUnblockTask_RejectsWrongDoerRole(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now)
	task.RolePair = "code-planning-pair"
	task.Worktree = testhelpers.StringPtr(".worktrees/task-1")
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	bb := db.New(stateFile)
	testhelpers.RegisterTestAgent(t, bb, "coder-1", "coder")

	_, err := UnblockTask(tmpDir, "task-1", "coder-1", "repair verified", "orchestrator-1")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "does not match task doer role") {
		t.Fatalf("Error = %q, want doer role mismatch", err.Error())
	}
}

func setAgentPID(t *testing.T, bb *db.Blackboard, agentID string, pid int) {
	t.Helper()
	err := bb.Modify(func(state *models.State) error {
		agent := state.Agents[agentID]
		agent.PID = pid
		state.Agents[agentID] = agent
		return nil
	})
	if err != nil {
		t.Fatalf("set agent PID: %v", err)
	}
}

func writeAndCommit(t *testing.T, repoDir, relPath, content, message string) {
	t.Helper()
	absPath := filepath.Join(repoDir, relPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		t.Fatalf("mkdir for %s: %v", relPath, err)
	}
	if err := os.WriteFile(absPath, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
	testhelpers.MustGit(t, repoDir, "add", relPath)
	testhelpers.MustGit(t, repoDir, "commit", "-m", message)
}
