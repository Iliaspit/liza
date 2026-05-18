package ops

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/errors"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestAssessBlocked_Validation(t *testing.T) {
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
			errContains: "only orchestrator agents can assess blocked tasks",
		},
		{
			name:        "reviewer agent ID",
			taskID:      "task-1",
			agentID:     "code-reviewer-1",
			errContains: "only orchestrator agents can assess blocked tasks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := AssessBlocked("/nonexistent", tt.taskID, "", tt.agentID)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Error = %q, want to contain %q", err.Error(), tt.errContains)
			}
		})
	}
}

func TestAssessBlocked_TaskNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := AssessBlocked(tmpDir, "nonexistent", "", "orchestrator-1")
	if err == nil {
		t.Fatal("Expected error for nonexistent task")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestAssessBlocked_WrongStatus(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := AssessBlocked(tmpDir, "task-1", "", "orchestrator-1")
	if err == nil {
		t.Fatal("Expected error for non-BLOCKED task")
	}
	if !strings.Contains(err.Error(), "BLOCKED status") {
		t.Errorf("Error = %q, want to contain 'BLOCKED status'", err.Error())
	}
}

func TestAssessBlocked_Success(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := AssessBlocked(tmpDir, "task-1", "Cannot resolve without external API", "orchestrator-1")
	if err != nil {
		t.Fatalf("AssessBlocked() error: %v", err)
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

	task := readState.FindTask("task-1")
	if task == nil {
		t.Fatal("Task not found")
	}
	// Status should remain BLOCKED
	if task.Status != models.TaskStatusBlocked {
		t.Errorf("Status = %v, want BLOCKED", task.Status)
	}

	lastHistory := task.History[len(task.History)-1]
	if lastHistory.Event != models.TaskEventOrchestratorAssessment {
		t.Errorf("History event = %q, want %q", lastHistory.Event, models.TaskEventOrchestratorAssessment)
	}
	if lastHistory.Agent == nil || *lastHistory.Agent != "orchestrator-1" {
		t.Errorf("Expected agent orchestrator-1 in history, got %v", lastHistory.Agent)
	}
	if lastHistory.Note == nil || *lastHistory.Note != "Cannot resolve without external API" {
		t.Errorf("Expected note in history, got %v", lastHistory.Note)
	}

	data, err := os.ReadFile(paths.New(tmpDir).AlertsLogPath())
	if err != nil {
		t.Fatalf("Read alerts.log: %v", err)
	}
	if !strings.Contains(string(data), "UNRESOLVED BLOCKED: task-1 — Cannot resolve without external API") {
		t.Fatalf("alerts.log missing unresolved alert:\n%s", string(data))
	}
}

func TestAssessBlocked_SuccessWithoutNote(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := AssessBlocked(tmpDir, "task-1", "", "orchestrator-1")
	if err != nil {
		t.Fatalf("AssessBlocked() error: %v", err)
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := readState.FindTask("task-1")
	lastHistory := task.History[len(task.History)-1]
	if lastHistory.Note != nil {
		t.Errorf("Expected nil note, got %v", lastHistory.Note)
	}
}

func TestAssessBlocked_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusBlocked, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	// First assessment
	_, err := AssessBlocked(tmpDir, "task-1", "first", "orchestrator-1")
	if err != nil {
		t.Fatalf("First AssessBlocked() error: %v", err)
	}

	// Second assessment
	_, err = AssessBlocked(tmpDir, "task-1", "second", "orchestrator-1")
	if err != nil {
		t.Fatalf("Second AssessBlocked() error: %v", err)
	}

	// Verify two entries
	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := readState.FindTask("task-1")
	assessmentCount := 0
	for _, entry := range task.History {
		if entry.Event == models.TaskEventOrchestratorAssessment {
			assessmentCount++
		}
	}
	if assessmentCount != 2 {
		t.Errorf("Expected 2 assessment entries, got %d", assessmentCount)
	}
}
