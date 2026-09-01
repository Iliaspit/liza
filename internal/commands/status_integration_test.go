package commands

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestStatusCommand_Integration(t *testing.T) {
	// Create a temporary directory for test state
	tmpDir, err := os.MkdirTemp("", "liza-status-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create the project runtime directory.
	lizaDir := paths.New(tmpDir).LizaDir()
	if err := os.MkdirAll(lizaDir, 0755); err != nil {
		t.Fatalf("failed to create project runtime directory: %v", err)
	}

	// Create a test state
	statePath := paths.New(tmpDir).StatePath()
	bb := db.New(statePath)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	state.Goal = models.Goal{
		ID:          "goal-1",
		Description: "Test goal for status command",
		SpecRef:     "specs/vision.md",
		Status:      models.GoalStatusInProgress,
		Created:     now,
	}
	state.Sprint = models.Sprint{
		ID:     "sprint-1",
		Status: models.SprintStatusInProgress,
		Timeline: models.SprintTimeline{
			Started: now,
		},
		Metrics: models.SprintMetrics{
			TasksDone: 2,
		},
	}
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
		testhelpers.BuildTaskByStatus("task-2", models.TaskStatusImplementing, now),
		testhelpers.BuildTaskByStatus("task-3", models.TaskStatusMerged, now),
		testhelpers.BuildTaskByStatus("task-4", models.TaskStatusMerged, now),
	}
	state.Agents = map[string]models.Agent{
		"coder-1": {
			Role:      "coder",
			Status:    models.AgentStatusWorking,
			Heartbeat: now.Add(-30 * time.Second),
			PID:       12345,
		},
	}
	state.Config.Mode = models.SystemModeRunning

	// Write the initial state
	if err := bb.Write(state); err != nil {
		t.Fatalf("failed to write state: %v", err)
	}

	tests := []struct {
		name           string
		opts           StatusOptions
		expectContains []string
	}{
		{
			name: "dashboard format",
			opts: StatusOptions{
				Format:      "",
				Detailed:    false,
				ProjectRoot: tmpDir,
			},
			expectContains: []string{
				"=== GOAL ===",
				"Test goal for status command",
				"=== SPRINT ===",
				"sprint-1",
				"Progress: 2/4 tasks complete",
				"=== SYSTEM ===",
				"Mode: RUNNING",
				"=== TASKS ===",
				"Total: 4",
				"=== AGENTS ===",
				"coder-1",
				"=== ORCHESTRATOR ===",
				"=== WORK QUEUES ===",
			},
		},
		{
			name: "json format",
			opts: StatusOptions{
				Format:      "json",
				Detailed:    false,
				ProjectRoot: tmpDir,
			},
			expectContains: []string{
				`"goal"`,
				`"sprint"`,
				`"config"`,
				`"tasks"`,
				`"agents"`,
				`"orchestrator_state"`,
				`"work_queues"`,
				`"Test goal for status command"`,
				`"sprint-1"`,
			},
		},
		{
			name: "yaml format",
			opts: StatusOptions{
				Format:      "yaml",
				Detailed:    false,
				ProjectRoot: tmpDir,
			},
			expectContains: []string{
				"goal:",
				"sprint:",
				"config:",
				"tasks:",
				"agents:",
				"orchestratorstate:", // YAML uses lowercase
				"workqueues:",        // YAML uses lowercase
				"Test goal for status command",
				"sprint-1",
			},
		},
		{
			name: "detailed mode with no anomalies",
			opts: StatusOptions{
				Format:      "",
				Detailed:    true,
				ProjectRoot: tmpDir,
			},
			expectContains: []string{
				"=== GOAL ===",
				"=== TASKS ===",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := StatusCommand(tt.opts)
			if err != nil {
				t.Fatalf("StatusCommand() error = %v", err)
			}

			// Check that all expected strings are present
			for _, expected := range tt.expectContains {
				if !strings.Contains(result, expected) {
					t.Errorf("expected output to contain %q, but it didn't.\nOutput:\n%s", expected, result)
				}
			}
		})
	}
}

func TestStatusCommand_ErrorCases(t *testing.T) {
	tests := []struct {
		name    string
		opts    StatusOptions
		wantErr bool
	}{
		{
			name: "nonexistent directory",
			opts: StatusOptions{
				Format:      "",
				Detailed:    false,
				ProjectRoot: "/nonexistent/directory",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := StatusCommand(tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("StatusCommand() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
