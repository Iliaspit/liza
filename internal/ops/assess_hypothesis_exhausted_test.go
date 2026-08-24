package ops

import (
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestAssessHypothesisExhausted_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		taskID      string
		agentID     string
		errContains string
	}{
		{
			name:        "empty task ID",
			agentID:     "orchestrator-1",
			errContains: "task ID is required",
		},
		{
			name:        "empty agent ID",
			taskID:      "task-1",
			errContains: "agent ID is required",
		},
		{
			name:        "non-orchestrator agent ID",
			taskID:      "task-1",
			agentID:     "coder-1",
			errContains: "only orchestrator agents can assess hypothesis-exhausted tasks",
		},
		{
			name:        "reviewer agent ID",
			taskID:      "task-1",
			agentID:     "code-reviewer-1",
			errContains: "only orchestrator agents can assess hypothesis-exhausted tasks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := AssessHypothesisExhausted("/nonexistent", tt.taskID, "", tt.agentID)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Error = %q, want to contain %q", err.Error(), tt.errContains)
			}
		})
	}
}

func TestAssessHypothesisExhausted_TaskNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := AssessHypothesisExhausted(tmpDir, "nonexistent", "", "orchestrator-1")
	if err == nil {
		t.Fatal("Expected error for nonexistent task")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestAssessHypothesisExhausted_InsufficientFailures(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
	task.FailedBy = []string{"coder-1"} // only 1, need 2+
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := AssessHypothesisExhausted(tmpDir, "task-1", "", "orchestrator-1")
	if err == nil {
		t.Fatal("Expected error for task with <2 failures")
	}
	if !strings.Contains(err.Error(), "failed_by") {
		t.Errorf("Error = %q, want to contain 'failed_by'", err.Error())
	}
}

func TestAssessHypothesisExhausted_TerminalStatus(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusMerged, now)
	task.FailedBy = []string{"coder-1", "coder-2"}
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := AssessHypothesisExhausted(tmpDir, "task-1", "", "orchestrator-1")
	if err == nil {
		t.Fatal("Expected error for terminal task")
	}
	if !strings.Contains(err.Error(), "terminal status") {
		t.Errorf("Error = %q, want to contain 'terminal status'", err.Error())
	}
}

func TestAssessHypothesisExhausted_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
	task.FailedBy = []string{"coder-1", "coder-2"}
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := AssessHypothesisExhausted(tmpDir, "task-1", "Needs spec revision", "orchestrator-1")
	if err != nil {
		t.Fatalf("AssessHypothesisExhausted() error: %v", err)
	}

	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}

	// Verify history entry
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	readTask := readState.FindTask("task-1")
	if readTask == nil {
		t.Fatal("Task not found")
	}
	if readTask.Status != models.TaskStatusBlocked {
		t.Errorf("Status = %v, want %v", readTask.Status, models.TaskStatusBlocked)
	}
	if readTask.BlockedReason == nil || !strings.Contains(*readTask.BlockedReason, "hypothesis_exhaustion") {
		t.Errorf("BlockedReason = %v, want hypothesis_exhaustion", readTask.BlockedReason)
	}
	if len(readTask.BlockedQuestions) == 0 {
		t.Fatal("BlockedQuestions is empty")
	}

	lastHistory := readTask.History[len(readTask.History)-1]
	if lastHistory.Event != models.TaskEventOrchestratorAssessment {
		t.Errorf("History event = %q, want %q", lastHistory.Event, models.TaskEventOrchestratorAssessment)
	}
	if lastHistory.Agent == nil || *lastHistory.Agent != "orchestrator-1" {
		t.Errorf("Expected agent orchestrator-1 in history, got %v", lastHistory.Agent)
	}
	if lastHistory.Note == nil || *lastHistory.Note != "Needs spec revision" {
		t.Errorf("Expected note in history, got %v", lastHistory.Note)
	}
}

func TestAssessHypothesisExhausted_SuccessWithoutNote(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
	task.FailedBy = []string{"coder-1", "coder-2"}
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := AssessHypothesisExhausted(tmpDir, "task-1", "", "orchestrator-1")
	if err != nil {
		t.Fatalf("AssessHypothesisExhausted() error: %v", err)
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	readTask := readState.FindTask("task-1")
	lastHistory := readTask.History[len(readTask.History)-1]
	if lastHistory.Note != nil {
		t.Errorf("Expected nil note, got %v", lastHistory.Note)
	}
}

func TestAssessHypothesisExhausted_Idempotent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now)
	task.FailedBy = []string{"coder-1", "coder-2"}
	state.Tasks = []models.Task{task}
	testhelpers.WriteInitialState(t, stateFile, state)

	// First assessment
	_, err := AssessHypothesisExhausted(tmpDir, "task-1", "first", "orchestrator-1")
	if err != nil {
		t.Fatalf("First AssessHypothesisExhausted() error: %v", err)
	}

	// Second assessment
	_, err = AssessHypothesisExhausted(tmpDir, "task-1", "second", "orchestrator-1")
	if err != nil {
		t.Fatalf("Second AssessHypothesisExhausted() error: %v", err)
	}

	// Verify two entries
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	readTask := readState.FindTask("task-1")
	assessmentCount := 0
	for _, entry := range readTask.History {
		if entry.Event == models.TaskEventOrchestratorAssessment {
			assessmentCount++
		}
	}
	if assessmentCount != 2 {
		t.Errorf("Expected 2 assessment entries, got %d", assessmentCount)
	}
}
