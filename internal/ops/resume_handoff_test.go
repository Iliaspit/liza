package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestResumeHandoff_Validation(t *testing.T) {
	t.Parallel()

	t.Run("empty agent ID", func(t *testing.T) {
		_, err := ResumeHandoff(ResumeHandoffInput{
			ProjectRoot: "/tmp",
			AgentID:     "",
		})
		if err == nil {
			t.Fatal("Expected error, got nil")
		}
		if !strings.Contains(err.Error(), "agent ID is required") {
			t.Errorf("Error = %q, want to contain 'agent ID is required'", err.Error())
		}
	})
}

func TestResumeHandoff_NoHandoffFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	worktree := ".worktrees/task-1"
	// Task is IMPLEMENTING but NOT handoff_pending
	state.Tasks = []models.Task{
		{
			ID:             "task-1",
			Status:         models.TaskStatusImplementing,
			AssignedTo:     testhelpers.StringPtr("coder-1"),
			Worktree:       &worktree,
			HandoffPending: false,
			Created:        now,
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ResumeHandoff(ResumeHandoffInput{
		ProjectRoot: tmpDir,
		AgentID:     "coder-1",
	})
	if err != nil {
		t.Fatalf("ResumeHandoff() error: %v", err)
	}

	if result.Found {
		t.Error("Found = true, want false")
	}
	if result.TaskID != "" {
		t.Errorf("TaskID = %q, want empty", result.TaskID)
	}
}

func TestResumeHandoff_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	agentID := "coder-1"
	taskID := "task-1"
	worktree := ".worktrees/task-1"
	expiredLease := time.Now().UTC().Add(-2 * time.Minute)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Config.LeaseDuration = 120
	state.Tasks = []models.Task{
		{
			ID:             taskID,
			Status:         models.TaskStatusImplementing,
			AssignedTo:     &agentID,
			Worktree:       &worktree,
			HandoffPending: true,
			LeaseExpires:   &expiredLease,
			History:        []models.TaskHistoryEntry{},
			Created:        now,
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	callStart := time.Now().UTC()
	result, err := ResumeHandoff(ResumeHandoffInput{
		ProjectRoot: tmpDir,
		AgentID:     agentID,
	})
	if err != nil {
		t.Fatalf("ResumeHandoff() error: %v", err)
	}

	if !result.Found {
		t.Fatal("Found = false, want true")
	}
	if result.TaskID != taskID {
		t.Errorf("TaskID = %q, want %q", result.TaskID, taskID)
	}
	if result.Worktree != worktree {
		t.Errorf("Worktree = %q, want %q", result.Worktree, worktree)
	}

	// Verify state was updated
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := readState.FindTask(taskID)
	if task == nil {
		t.Fatalf("Task %q not found after resume", taskID)
	}
	if task.HandoffPending {
		t.Error("HandoffPending = true, want false")
	}
	if task.LeaseExpires == nil || !task.LeaseExpires.After(callStart) {
		t.Errorf("LeaseExpires = %v, want renewed lease after %v", task.LeaseExpires, callStart)
	}
	if len(task.History) == 0 || task.History[len(task.History)-1].Event != models.TaskEventHandoffResumed {
		t.Errorf("Last history event = %v, want %s", task.History, models.TaskEventHandoffResumed)
	}

	agent, ok := readState.Agents[agentID]
	if !ok {
		t.Fatalf("Agent %q not found after resume", agentID)
	}
	if agent.Status != models.AgentStatusWorking {
		t.Errorf("Agent status = %q, want %q", agent.Status, models.AgentStatusWorking)
	}
	if agent.CurrentTask == nil || *agent.CurrentTask != taskID {
		t.Errorf("Agent CurrentTask = %v, want %q", agent.CurrentTask, taskID)
	}
}

func TestResumeHandoff_MissingWorktree(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	agentID := "coder-1"
	state := testhelpers.CreateValidState()
	// IMPLEMENTING task without Worktree field
	state.Tasks = []models.Task{
		{
			ID:             "task-1",
			Status:         models.TaskStatusImplementing,
			AssignedTo:     &agentID,
			HandoffPending: true,
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := ResumeHandoff(ResumeHandoffInput{
		ProjectRoot: tmpDir,
		AgentID:     agentID,
	})
	if err == nil {
		t.Fatal("Expected error for missing worktree, got nil")
	}
	if !strings.Contains(err.Error(), "missing worktree") {
		t.Errorf("Error = %q, want to contain 'missing worktree'", err.Error())
	}
}

func TestResumeHandoff_WrongAgent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	worktree := ".worktrees/task-1"
	otherAgent := "coder-2"
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		{
			ID:             "task-1",
			Status:         models.TaskStatusImplementing,
			AssignedTo:     &otherAgent, // Assigned to different agent
			Worktree:       &worktree,
			HandoffPending: true,
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ResumeHandoff(ResumeHandoffInput{
		ProjectRoot: tmpDir,
		AgentID:     "coder-1", // Trying to claim as different agent
	})
	if err != nil {
		t.Fatalf("ResumeHandoff() error: %v", err)
	}

	// Should return not found (not an error) when task is assigned to different agent
	if result.Found {
		t.Error("Found = true, want false when task assigned to different agent")
	}
}

func TestResumeHandoff_TaskNotImplementing(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	agentID := "coder-1"
	worktree := ".worktrees/task-1"
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		{
			ID:             "task-1",
			Status:         models.TaskStatusReady, // Not IMPLEMENTING
			AssignedTo:     &agentID,
			Worktree:       &worktree,
			HandoffPending: true,
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := ResumeHandoff(ResumeHandoffInput{
		ProjectRoot: tmpDir,
		AgentID:     agentID,
	})
	if err != nil {
		t.Fatalf("ResumeHandoff() error: %v", err)
	}

	// Should return not found when task is not IMPLEMENTING
	if result.Found {
		t.Error("Found = true, want false when task not IMPLEMENTING")
	}
}

func TestResumeHandoff_StaleCandidateSkipped(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	agentID := "coder-1"
	worktree1 := ".worktrees/task-1"
	worktree2 := ".worktrees/task-2"
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		{
			ID:             "task-1",
			Status:         models.TaskStatusImplementing,
			AssignedTo:     &agentID,
			Worktree:       &worktree1,
			HandoffPending: true,
		},
		{
			ID:             "task-2",
			Status:         models.TaskStatusImplementing,
			AssignedTo:     &agentID,
			Worktree:       &worktree2,
			HandoffPending: true,
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	// Make first task stale after state is read
	bb := db.New(stateFile)
	_ = bb.Modify(func(s *models.State) error {
		t1 := s.FindTask("task-1")
		if t1 != nil {
			t1.Status = models.TaskStatusReady // No longer IMPLEMENTING
		}
		return nil
	})

	result, err := ResumeHandoff(ResumeHandoffInput{
		ProjectRoot: tmpDir,
		AgentID:     agentID,
	})
	if err != nil {
		t.Fatalf("ResumeHandoff() error: %v", err)
	}

	if !result.Found {
		t.Fatal("Found = false, want true")
	}
	// Should have skipped the stale task-1 and resumed task-2
	if result.TaskID != "task-2" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-2")
	}
}

func TestResumeHandoff_AlwaysRefreshesLease(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	agentID := "coder-1"
	worktree := ".worktrees/task-1"
	leaseDuration := 120
	// Lease is still valid (expires in 1 hour) but should be refreshed anyway
	validLease := time.Now().UTC().Add(1 * time.Hour)

	state := testhelpers.CreateValidState()
	state.Config.LeaseDuration = leaseDuration
	state.Tasks = []models.Task{
		{
			ID:             "task-1",
			Status:         models.TaskStatusImplementing,
			AssignedTo:     &agentID,
			Worktree:       &worktree,
			HandoffPending: true,
			LeaseExpires:   &validLease,
			History:        []models.TaskHistoryEntry{},
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	callStart := time.Now().UTC()
	result, err := ResumeHandoff(ResumeHandoffInput{
		ProjectRoot: tmpDir,
		AgentID:     agentID,
	})
	if err != nil {
		t.Fatalf("ResumeHandoff() error: %v", err)
	}

	if !result.Found {
		t.Fatal("Found = false, want true")
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

	// Lease should be unconditionally refreshed (not preserved at old value)
	expectedMin := callStart.Add(time.Duration(leaseDuration) * time.Second)
	if task.LeaseExpires == nil {
		t.Fatal("LeaseExpires is nil, want refreshed lease")
	}
	if task.LeaseExpires.Before(expectedMin) {
		t.Errorf("LeaseExpires = %v, want >= %v (fresh lease from callStart)", task.LeaseExpires, expectedMin)
	}
	// The old 1-hour lease should have been replaced with a 2-minute lease
	if task.LeaseExpires.Equal(validLease) {
		t.Error("LeaseExpires still equals old valid lease — should have been refreshed")
	}
}
