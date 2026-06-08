package commands

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestSprintCheckpointCommand(t *testing.T) {
	tests := []struct {
		name             string
		initialStatus    models.SprintStatus
		tasks            []models.Task
		wantErr          bool
		errContains      string
		wantStatus       models.SprintStatus
		wantReportExists bool
	}{
		{
			name:             "checkpoint from IN_PROGRESS",
			initialStatus:    models.SprintStatusInProgress,
			tasks:            []models.Task{},
			wantErr:          false,
			wantStatus:       models.SprintStatusCheckpoint,
			wantReportExists: true,
		},
		{
			name:          "cannot checkpoint when already at CHECKPOINT",
			initialStatus: models.SprintStatusCheckpoint,
			tasks:         []models.Task{},
			wantErr:       true,
			errContains:   "already at CHECKPOINT",
			wantStatus:    models.SprintStatusCheckpoint,
		},
		{
			name:          "cannot checkpoint when COMPLETED",
			initialStatus: models.SprintStatusCompleted,
			tasks:         []models.Task{},
			wantErr:       true,
			errContains:   "cannot checkpoint",
			wantStatus:    models.SprintStatusCompleted,
		},
		{
			name:          "cannot checkpoint when ABORTED",
			initialStatus: models.SprintStatusAborted,
			tasks:         []models.Task{},
			wantErr:       true,
			errContains:   "cannot checkpoint",
			wantStatus:    models.SprintStatusAborted,
		},
		{
			name:          "checkpoint with tasks generates report",
			initialStatus: models.SprintStatusInProgress,
			tasks: []models.Task{
				{
					ID:          "task-1",
					Status:      models.TaskStatusMerged,
					Description: "Done task",
					Created:     time.Now(),
					SpecRef:     "spec.md",
					DoneWhen:    "Done",
					Scope:       "Test",
					History:     []models.TaskHistoryEntry{},
				},
				{
					ID:          "task-2",
					Status:      models.TaskStatusImplementing,
					Description: "In progress task",
					Created:     time.Now(),
					SpecRef:     "spec.md",
					DoneWhen:    "Done",
					Scope:       "Test",
					History:     []models.TaskHistoryEntry{},
				},
			},
			wantErr:          false,
			wantStatus:       models.SprintStatusCheckpoint,
			wantReportExists: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			tmpDir := t.TempDir()
			stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
			testhelpers.SetupPipelineConfig(t, tmpDir)
			lizaDir := paths.New(tmpDir).LizaDir()

			// Create initial state with specified status
			state := testhelpers.CreateValidState()
			state.Sprint.Status = tt.initialStatus
			state.Tasks = tt.tasks

			bb := testhelpers.WriteInitialState(t, stateFile, state)

			// Run checkpoint command
			err := SprintCheckpointCommand(tmpDir)

			// Check error
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				testhelpers.AssertErrorContains(t, err, tt.errContains)

				// Verify status didn't change
				updatedState, readErr := bb.Read()
				if readErr != nil {
					t.Fatalf("Failed to read state: %v", readErr)
				}
				if updatedState.Sprint.Status != tt.wantStatus {
					t.Errorf("Status should not have changed: got %v, want %v", updatedState.Sprint.Status, tt.wantStatus)
				}
				return
			}

			testhelpers.AssertNoError(t, err)

			// Verify sprint status was updated
			updatedState, err := bb.Read()
			if err != nil {
				t.Fatalf("Failed to read state: %v", err)
			}

			if updatedState.Sprint.Status != tt.wantStatus {
				t.Errorf("Sprint status = %v, want %v", updatedState.Sprint.Status, tt.wantStatus)
			}

			// Verify checkpoint_at was set
			if updatedState.Sprint.Timeline.CheckpointAt == nil {
				t.Error("CheckpointAt should be set")
			}

			// Verify report exists
			reportPath := filepath.Join(lizaDir, "sprint_summary.md")
			_, err = os.Stat(reportPath)
			reportExists := err == nil
			if reportExists != tt.wantReportExists {
				t.Errorf("Report exists = %v, want %v", reportExists, tt.wantReportExists)
			}

			// Verify report content
			if tt.wantReportExists && reportExists {
				reportData, err := os.ReadFile(reportPath)
				if err != nil {
					t.Fatalf("Failed to read report: %v", err)
				}

				report := string(reportData)
				expectedSections := []string{
					"# Sprint Summary",
					"## Sprint Status",
					"## Task Status",
					"## Sprint Metrics",
				}

				for _, section := range expectedSections {
					if !strings.Contains(report, section) {
						t.Errorf("Report missing expected section: %q", section)
					}
				}
			}
		})
	}
}

func TestSprintCheckpointCommandTransitionCheckpointOutput(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
	testhelpers.SetupPipelineConfig(t, tmpDir)

	state := testhelpers.CreateValidState()
	state.Sprint.Status = models.SprintStatusInProgress
	state.Tasks = []models.Task{
		func() models.Task {
			task := testhelpers.BuildTaskByStatus("plan-ready", models.TaskStatusMerged, time.Now().UTC())
			task.RolePair = "code-planning-pair"
			task.Output = []models.OutputEntry{
				{Desc: "implement X", DoneWhen: "tests pass", Scope: "pkg/x"},
			}
			return task
		}(),
	}
	state.Sprint.Scope.Planned = []string{"plan-ready"}
	testhelpers.WriteInitialState(t, stateFile, state)

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe stdout: %v", err)
	}
	os.Stdout = w
	err = SprintCheckpointCommand(tmpDir)
	w.Close()
	os.Stdout = oldStdout
	if err != nil {
		t.Fatalf("SprintCheckpointCommand() error = %v", err)
	}
	outBytes, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	out := string(outBytes)
	if !strings.Contains(out, "Transition gate is pending") {
		t.Fatalf("stdout missing transition gate guidance:\n%s", out)
	}
	if strings.Contains(out, "Agents will pause at their next check") {
		t.Fatalf("stdout used hard-checkpoint pause guidance for transition checkpoint:\n%s", out)
	}
}
