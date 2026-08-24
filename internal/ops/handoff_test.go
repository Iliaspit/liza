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

func TestHandoff_Validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       *HandoffInput
		errContains string
	}{
		{
			name:        "empty task ID",
			input:       &HandoffInput{Summary: "s", NextAction: "n", AgentID: "a"},
			errContains: "task ID is required",
		},
		{
			name:        "empty summary",
			input:       &HandoffInput{TaskID: "t1", NextAction: "n", AgentID: "a"},
			errContains: "summary is required",
		},
		{
			name:        "empty next action",
			input:       &HandoffInput{TaskID: "t1", Summary: "s", AgentID: "a"},
			errContains: "next action is required",
		},
		{
			name:        "empty agent ID",
			input:       &HandoffInput{TaskID: "t1", Summary: "s", NextAction: "n"},
			errContains: "LIZA_AGENT_ID is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.input.ProjectRoot = "/nonexistent"
			_, err := Handoff(tt.input)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.errContains) {
				t.Errorf("Error = %q, want to contain %q", err.Error(), tt.errContains)
			}
		})
	}
}

func TestHandoff_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Handoff(&HandoffInput{
		ProjectRoot: tmpDir,
		TaskID:      "task-1",
		Summary:     "Context at 80%",
		NextAction:  "Continue from function X",
		AgentID:     "coder-1",
	})
	if err != nil {
		t.Fatalf("Handoff() error: %v", err)
	}

	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}
	if result.AgentID != "coder-1" {
		t.Errorf("AgentID = %q, want %q", result.AgentID, "coder-1")
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
	if !task.HandoffPending {
		t.Error("HandoffPending should be true")
	}

	// Verify handoff event appended to task (backward compat: summary→succeeded)
	if len(task.HandoffEvents) != 1 {
		t.Fatalf("HandoffEvents count = %d, want 1", len(task.HandoffEvents))
	}
	he := task.HandoffEvents[0]
	if he.Agent != "coder-1" {
		t.Errorf("HandoffEvent.Agent = %q, want %q", he.Agent, "coder-1")
	}
	if he.Trigger != models.HandoffTriggerContextExhaustion {
		t.Errorf("HandoffEvent.Trigger = %q, want %q", he.Trigger, models.HandoffTriggerContextExhaustion)
	}
	if len(he.Succeeded) != 1 || he.Succeeded[0] != "Context at 80%" {
		t.Errorf("HandoffEvent.Succeeded = %v, want [%q]", he.Succeeded, "Context at 80%")
	}
	if he.NextStep != "Continue from function X" {
		t.Errorf("HandoffEvent.NextStep = %q, want %q", he.NextStep, "Continue from function X")
	}
	if he.Timestamp.IsZero() {
		t.Error("HandoffEvent.Timestamp should not be zero")
	}

	// Verify agent status
	agent, exists := readState.Agents["coder-1"]
	if !exists {
		t.Fatal("Agent not found")
	}
	if agent.Status != models.AgentStatusHandoff {
		t.Errorf("Agent status = %v, want HANDOFF", agent.Status)
	}
	if agent.CurrentTask == nil || *agent.CurrentTask != "task-1" {
		t.Error("Agent CurrentTask should be task-1")
	}

	// Verify history
	lastHistory := task.History[len(task.History)-1]
	if lastHistory.Event != models.TaskEventHandoffInitiated {
		t.Errorf("History event = %q, want %q", lastHistory.Event, models.TaskEventHandoffInitiated)
	}
}

func TestHandoff_StructuredFields(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Handoff(&HandoffInput{
		ProjectRoot: tmpDir,
		TaskID:      "task-1",
		Summary:     "legacy summary",
		NextAction:  "next step action",
		AgentID:     "coder-1",
		Succeeded:   []string{"parsed input", "validated schema"},
		Failed:      []string{"edge case for empty input"},
		Hypothesis:  "empty input triggers nil pointer",
		KeyFiles:    []string{"internal/parser.go", "internal/validate.go"},
		DeadEnds:    []string{"tried regex approach, too fragile"},
	})
	if err != nil {
		t.Fatalf("Handoff() error: %v", err)
	}
	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}

	bb := db.New(stateFile)
	readState, err := bb.Read()
	if err != nil {
		t.Fatalf("Failed to read state: %v", err)
	}

	task := readState.FindTask("task-1")
	if task == nil {
		t.Fatal("Task not found")
	}

	if len(task.HandoffEvents) != 1 {
		t.Fatalf("HandoffEvents count = %d, want 1", len(task.HandoffEvents))
	}
	he := task.HandoffEvents[0]

	// Succeeded should use explicit value, not legacy summary
	if len(he.Succeeded) != 2 || he.Succeeded[0] != "parsed input" || he.Succeeded[1] != "validated schema" {
		t.Errorf("HandoffEvent.Succeeded = %v, want [parsed input, validated schema]", he.Succeeded)
	}
	if len(he.Failed) != 1 || he.Failed[0] != "edge case for empty input" {
		t.Errorf("HandoffEvent.Failed = %v, want [edge case for empty input]", he.Failed)
	}
	if he.Hypothesis != "empty input triggers nil pointer" {
		t.Errorf("HandoffEvent.Hypothesis = %q, want %q", he.Hypothesis, "empty input triggers nil pointer")
	}
	if he.NextStep != "next step action" {
		t.Errorf("HandoffEvent.NextStep = %q, want %q", he.NextStep, "next step action")
	}
	if len(he.KeyFiles) != 2 || he.KeyFiles[0] != "internal/parser.go" {
		t.Errorf("HandoffEvent.KeyFiles = %v, want [internal/parser.go, internal/validate.go]", he.KeyFiles)
	}
	if len(he.DeadEnds) != 1 || he.DeadEnds[0] != "tried regex approach, too fragile" {
		t.Errorf("HandoffEvent.DeadEnds = %v, want [tried regex approach, too fragile]", he.DeadEnds)
	}
}

func TestHandoff_TaskNotFound(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	state := testhelpers.CreateValidState()
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Handoff(&HandoffInput{
		ProjectRoot: tmpDir,
		TaskID:      "nonexistent",
		Summary:     "s",
		NextAction:  "n",
		AgentID:     "coder-1",
	})
	if err == nil {
		t.Fatal("Expected error for nonexistent task")
	}
	if !errors.IsNotFound(err) {
		t.Errorf("expected NotFoundError, got %T: %v", err, err)
	}
}

func TestHandoff_WrongStatus(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Handoff(&HandoffInput{
		ProjectRoot: tmpDir,
		TaskID:      "task-1",
		Summary:     "s",
		NextAction:  "n",
		AgentID:     "coder-1",
	})
	if err == nil {
		t.Fatal("Expected error for non-executing task")
	}
	if !strings.Contains(err.Error(), "not in an executing status") {
		t.Errorf("Error = %q, want to contain 'not in an executing status'", err.Error())
	}
}

func TestHandoff_PipelineExecutingStatus(t *testing.T) {
	t.Parallel()

	tmpDir, stateFile := setupPipelineTest(t)

	now := time.Now().UTC()
	agent := "code-planner-1"
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		{
			ID:          "task-1",
			Type:        models.TaskTypeCoding,
			Description: "Pipeline task",
			Status:      "CODE_PLANNING", // pipeline executing status
			Priority:    1,
			Created:     now,
			SpecRef:     "README.md",
			DoneWhen:    "Done",
			Scope:       "Scope",
			AssignedTo:  &agent,
			History:     []models.TaskHistoryEntry{},
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	result, err := Handoff(&HandoffInput{
		ProjectRoot: tmpDir,
		TaskID:      "task-1",
		Summary:     "Context exhausted",
		NextAction:  "Resume from step 3",
		AgentID:     agent,
	})
	if err != nil {
		t.Fatalf("Handoff() error: %v", err)
	}
	if result.TaskID != "task-1" {
		t.Errorf("TaskID = %q, want %q", result.TaskID, "task-1")
	}
}

func TestHandoff_PipelineNonExecutingStatus(t *testing.T) {
	t.Parallel()

	tmpDir, stateFile := setupPipelineTest(t)

	now := time.Now().UTC()
	agent := "code-planner-1"
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		{
			ID:          "task-1",
			Type:        models.TaskTypeCoding,
			Description: "Pipeline task",
			Status:      "CODING_PLAN_TO_REVIEW", // pipeline submitted status, not executing
			Priority:    1,
			Created:     now,
			SpecRef:     "README.md",
			DoneWhen:    "Done",
			Scope:       "Scope",
			AssignedTo:  &agent,
			History:     []models.TaskHistoryEntry{},
		},
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Handoff(&HandoffInput{
		ProjectRoot: tmpDir,
		TaskID:      "task-1",
		Summary:     "s",
		NextAction:  "n",
		AgentID:     agent,
	})
	if err == nil {
		t.Fatal("Expected error for non-executing pipeline status")
	}
	if !strings.Contains(err.Error(), "not in an executing status") {
		t.Errorf("Error = %q, want to contain 'not in an executing status'", err.Error())
	}
}

func TestHandoff_WrongAgent(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	_, err := Handoff(&HandoffInput{
		ProjectRoot: tmpDir,
		TaskID:      "task-1",
		Summary:     "s",
		NextAction:  "n",
		AgentID:     "coder-2",
	})
	if err == nil {
		t.Fatal("Expected error for wrong agent")
	}
	if !strings.Contains(err.Error(), "not assigned to agent") {
		t.Errorf("Error = %q, want to contain 'not assigned to agent'", err.Error())
	}
}
