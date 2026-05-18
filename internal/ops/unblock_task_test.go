package ops

import (
	"os"
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

func TestUnblockTask_RejectsAssignToWithoutLiveProcess(t *testing.T) {
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

	_, err := UnblockTask(tmpDir, "task-1", "code-planner-1", "repair verified", "orchestrator-1")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "has no live process") {
		t.Fatalf("Error = %q, want no live process", err.Error())
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
