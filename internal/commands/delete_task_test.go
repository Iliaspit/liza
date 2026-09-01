package commands

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/git"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestDeleteTaskCommand(t *testing.T) {
	tests := []struct {
		name             string
		taskID           string
		force            bool
		deleteWorktree   bool
		reason           string
		stdinInput       string // For interactive prompt testing
		setupState       func(*models.State)
		setupWorktree    bool // Whether to create actual worktree
		wantErr          bool
		wantErrMsg       string
		validateState    func(*testing.T, *models.State)
		validateWorktree func(*testing.T, string, string) // projectRoot, taskID
	}{
		{
			name:           "successfully delete DRAFT task",
			taskID:         "task-1",
			force:          false,
			deleteWorktree: false,
			reason:         "test deletion",
			setupState: func(state *models.State) {
				state.Tasks = append(state.Tasks, testhelpers.BuildTaskByStatus("task-1", models.TaskStatusDraft, time.Now().UTC()))
			},
			wantErr: false,
			validateState: func(t *testing.T, state *models.State) {
				for _, task := range state.Tasks {
					if task.ID == "task-1" {
						t.Error("Task should have been deleted")
					}
				}
			},
		},
		{
			name:           "successfully delete READY task",
			taskID:         "task-2",
			force:          false,
			deleteWorktree: false,
			reason:         "not needed",
			setupState: func(state *models.State) {
				state.Tasks = append(state.Tasks, testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReady, time.Now().UTC()))
			},
			wantErr: false,
			validateState: func(t *testing.T, state *models.State) {
				for _, task := range state.Tasks {
					if task.ID == "task-2" {
						t.Error("Task should have been deleted")
					}
				}
			},
		},
		{
			name:           "successfully delete ABANDONED task",
			taskID:         "task-3",
			force:          false,
			deleteWorktree: false,
			reason:         "cleanup",
			setupState: func(state *models.State) {
				state.Tasks = append(state.Tasks, testhelpers.BuildTaskByStatus("task-3", models.TaskStatusAbandoned, time.Now().UTC()))
			},
			wantErr: false,
		},
		{
			name:           "successfully delete SUPERSEDED task",
			taskID:         "task-4",
			force:          false,
			deleteWorktree: false,
			reason:         "cleanup",
			setupState: func(state *models.State) {
				state.Tasks = append(state.Tasks, testhelpers.BuildTaskByStatus("task-4", models.TaskStatusSuperseded, time.Now().UTC()))
			},
			wantErr: false,
		},
		{
			name:           "successfully delete INTEGRATION_FAILED task",
			taskID:         "task-5",
			force:          false,
			deleteWorktree: false,
			reason:         "cleanup",
			setupState: func(state *models.State) {
				state.Tasks = append(state.Tasks, testhelpers.BuildTaskByStatus("task-5", models.TaskStatusIntegrationFailed, time.Now().UTC()))
			},
			wantErr: false,
		},
		{
			name:           "error when deleting MERGED task without force",
			taskID:         "task-6",
			force:          false,
			deleteWorktree: false,
			reason:         "test",
			setupState: func(state *models.State) {
				state.Tasks = append(state.Tasks, testhelpers.BuildTaskByStatus("task-6", models.TaskStatusMerged, time.Now().UTC()))
			},
			wantErr:    true,
			wantErrMsg: "cannot delete MERGED task",
		},
		{
			name:           "force delete MERGED task",
			taskID:         "task-7",
			force:          true,
			deleteWorktree: false,
			reason:         "forced deletion",
			setupState: func(state *models.State) {
				state.Tasks = append(state.Tasks, testhelpers.BuildTaskByStatus("task-7", models.TaskStatusMerged, time.Now().UTC()))
			},
			wantErr: false,
		},
		{
			name:           "error when deleting IMPLEMENTING task with valid lease",
			taskID:         "task-8",
			force:          false,
			deleteWorktree: false,
			reason:         "test",
			setupState: func(state *models.State) {
				task := testhelpers.BuildTaskByStatus("task-8", models.TaskStatusImplementing, time.Now().UTC())
				validLease := time.Now().UTC().Add(1 * time.Hour)
				task.LeaseExpires = &validLease
				state.Tasks = append(state.Tasks, task)
			},
			wantErr:    true,
			wantErrMsg: "cannot delete task",
		},
		{
			name:           "error when deleting READY_FOR_REVIEW task",
			taskID:         "task-9",
			force:          false,
			deleteWorktree: false,
			reason:         "test",
			setupState: func(state *models.State) {
				state.Tasks = append(state.Tasks, testhelpers.BuildTaskByStatus("task-9", models.TaskStatusReadyForReview, time.Now().UTC()))
			},
			wantErr:    true,
			wantErrMsg: "cannot delete task",
		},
		{
			name:           "error when deleting REVIEWING task",
			taskID:         "task-9b",
			force:          false,
			deleteWorktree: false,
			reason:         "test",
			setupState: func(state *models.State) {
				state.Tasks = append(state.Tasks, testhelpers.BuildTaskByStatus("task-9b", models.TaskStatusReviewing, time.Now().UTC()))
			},
			wantErr:    true,
			wantErrMsg: "cannot delete task",
		},
		{
			name:           "delete task without worktree",
			taskID:         "task-10",
			force:          false,
			deleteWorktree: false,
			reason:         "test",
			setupState: func(state *models.State) {
				task := testhelpers.BuildTaskByStatus("task-10", models.TaskStatusDraft, time.Now().UTC())
				task.Worktree = nil
				state.Tasks = append(state.Tasks, task)
			},
			wantErr: false,
		},
		{
			name:           "error when dependent tasks exist without force",
			taskID:         "task-11",
			force:          false,
			deleteWorktree: false,
			reason:         "test",
			setupState: func(state *models.State) {
				state.Tasks = append(state.Tasks, testhelpers.BuildTaskByStatus("task-11", models.TaskStatusReady, time.Now().UTC()))
				dependentTask := testhelpers.BuildTaskByStatus("task-12", models.TaskStatusReady, time.Now().UTC())
				dependentTask.DependsOn = []string{"task-11"}
				state.Tasks = append(state.Tasks, dependentTask)
			},
			wantErr:    true,
			wantErrMsg: "depend on it",
		},
		{
			name:           "force delete task with dependents",
			taskID:         "task-13",
			force:          true,
			deleteWorktree: false,
			reason:         "forced",
			setupState: func(state *models.State) {
				state.Tasks = append(state.Tasks, testhelpers.BuildTaskByStatus("task-13", models.TaskStatusReady, time.Now().UTC()))
				dependentTask := testhelpers.BuildTaskByStatus("task-14", models.TaskStatusReady, time.Now().UTC())
				dependentTask.DependsOn = []string{"task-13"}
				state.Tasks = append(state.Tasks, dependentTask)
			},
			wantErr: false,
		},
		{
			name:           "error on empty task ID",
			taskID:         "",
			force:          false,
			deleteWorktree: false,
			reason:         "test",
			setupState:     func(state *models.State) {},
			wantErr:        true,
			wantErrMsg:     "task ID required",
		},
		{
			name:           "error on nonexistent task",
			taskID:         "nonexistent",
			force:          false,
			deleteWorktree: false,
			reason:         "test",
			setupState:     func(state *models.State) {},
			wantErr:        true,
			wantErrMsg:     "task not found",
		},
		{
			name:           "verify task removed from sprint planned",
			taskID:         "task-15",
			force:          false,
			deleteWorktree: false,
			reason:         "test",
			setupState: func(state *models.State) {
				state.Tasks = append(state.Tasks, testhelpers.BuildTaskByStatus("task-15", models.TaskStatusReady, time.Now().UTC()))
				state.Sprint.Scope.Planned = append(state.Sprint.Scope.Planned, "task-15")
			},
			wantErr: false,
			validateState: func(t *testing.T, state *models.State) {
				for _, taskID := range state.Sprint.Scope.Planned {
					if taskID == "task-15" {
						t.Error("Task should have been removed from sprint.scope.planned")
					}
				}
			},
		},
		{
			name:           "verify task removed from sprint stretch",
			taskID:         "task-16",
			force:          false,
			deleteWorktree: false,
			reason:         "test",
			setupState: func(state *models.State) {
				state.Tasks = append(state.Tasks, testhelpers.BuildTaskByStatus("task-16", models.TaskStatusReady, time.Now().UTC()))
				state.Sprint.Scope.Stretch = append(state.Sprint.Scope.Stretch, "task-16")
			},
			wantErr: false,
			validateState: func(t *testing.T, state *models.State) {
				for _, taskID := range state.Sprint.Scope.Stretch {
					if taskID == "task-16" {
						t.Error("Task should have been removed from sprint.scope.stretch")
					}
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup temp directory
			tmpDir := t.TempDir()

			// Create the project runtime directory.
			lizaDir := paths.New(tmpDir).LizaDir()
			if err := os.MkdirAll(lizaDir, 0755); err != nil {
				t.Fatal(err)
			}

			// Initialize git repo if worktree testing is needed
			if tt.setupWorktree {
				// Initialize git repo
				_ = git.New(tmpDir)
				if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("test"), 0644); err != nil {
					t.Fatal(err)
				}
				// Git commands need to be run - skip for now in tests unless really needed
			}

			// Create state file
			statePath := filepath.Join(lizaDir, paths.StateFileName)
			state := testhelpers.CreateValidState()

			// Setup state with test data
			tt.setupState(state)

			// Write initial state
			bb := db.New(statePath)
			if err := bb.Write(state); err != nil {
				t.Fatal(err)
			}

			// Setup stdin for interactive prompts
			var stdin io.Reader = nil
			if tt.stdinInput != "" {
				stdin = strings.NewReader(tt.stdinInput)
			}

			// Execute command
			err := DeleteTaskCommand(tmpDir, tt.taskID, tt.force, tt.deleteWorktree, tt.reason, stdin)

			// Check error
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error but got none")
				} else if tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.wantErrMsg, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Validate final state
			finalState, err := bb.Read()
			if err != nil {
				t.Fatal(err)
			}

			// Default validation: task should be deleted
			taskFound := false
			for _, task := range finalState.Tasks {
				if task.ID == tt.taskID {
					taskFound = true
					break
				}
			}
			if taskFound {
				t.Errorf("Task %s should have been deleted", tt.taskID)
			}

			// Custom validation
			if tt.validateState != nil {
				tt.validateState(t, finalState)
			}

			// Validate worktree if needed
			if tt.validateWorktree != nil {
				tt.validateWorktree(t, tmpDir, tt.taskID)
			}
		})
	}
}

// TestDeleteTaskCommand_APPROVED tests the special warning for APPROVED tasks
func TestDeleteTaskCommand_APPROVED(t *testing.T) {
	tmpDir := t.TempDir()

	// Create the project runtime directory.
	lizaDir := filepath.Join(tmpDir, paths.LizaDirName)
	if err := os.MkdirAll(lizaDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create state file
	statePath := filepath.Join(lizaDir, paths.StateFileName)
	state := testhelpers.CreateValidState()

	// Add APPROVED task
	state.Tasks = append(state.Tasks, testhelpers.BuildTaskByStatus("task-approved", models.TaskStatusApproved, time.Now().UTC()))

	// Write initial state
	bb := db.New(statePath)
	if err := bb.Write(state); err != nil {
		t.Fatal(err)
	}

	// Capture stderr to check for warning
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// Mock stdin to answer 'y' to confirmation (two 'y's - one for APPROVED warning, one for worktree prompt)
	stdin := strings.NewReader("y\ny\n")

	// Execute command (should prompt with warning)
	err := DeleteTaskCommand(tmpDir, "task-approved", false, false, "test", stdin)

	// Restore stderr
	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// Check that warning was shown
	if !strings.Contains(output, "has been implemented, reviewed, and approved") {
		t.Errorf("Expected APPROVED task warning in output, got: %s", output)
	}

	// Verify task was deleted
	finalState, err := bb.Read()
	if err != nil {
		t.Fatal(err)
	}

	for _, task := range finalState.Tasks {
		if task.ID == "task-approved" {
			t.Error("APPROVED task should have been deleted after confirmation")
		}
	}
}
