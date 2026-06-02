package commands

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/log"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestClearStaleReviewClaimsCommand(t *testing.T) {
	tests := []struct {
		name         string
		tasks        []models.Task
		wantCleared  int
		wantErr      bool
		errContains  string
		validateFunc func(*testing.T, *models.State, []log.Entry)
	}{
		{
			name: "clear single stale review claim",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, time.Now().UTC())
					// Set lease to 1 hour in the past (expired)
					expiredTime := time.Now().UTC().Add(-1 * time.Hour)
					task.ReviewLeaseExpires = &expiredTime
					return task
				}(),
			},
			wantCleared: 1,
			wantErr:     false,
			validateFunc: func(t *testing.T, state *models.State, entries []log.Entry) {
				task := &state.Tasks[0]
				if task.ReviewingBy != nil {
					t.Errorf("ReviewingBy should be nil, got %v", *task.ReviewingBy)
				}
				if task.ReviewLeaseExpires != nil {
					t.Errorf("ReviewLeaseExpires should be nil, got %v", *task.ReviewLeaseExpires)
				}
				if task.Status != models.TaskStatusReadyForReview {
					t.Errorf("Status should be READY_FOR_REVIEW, got %s", task.Status)
				}

				// Check log entry
				if len(entries) == 0 {
					t.Fatal("Expected log entry")
				}
				lastEntry := entries[len(entries)-1]
				if lastEntry.Action != "stale_review_cleared" {
					t.Errorf("Log action = %q, want %q", lastEntry.Action, "stale_review_cleared")
				}
				if lastEntry.Task == nil || *lastEntry.Task != "task-1" {
					t.Errorf("Log task = %v, want %q", lastEntry.Task, "task-1")
				}
				if lastEntry.Agent != "system" {
					t.Errorf("Log agent = %q, want %q", lastEntry.Agent, "system")
				}
			},
		},
		{
			name: "clear multiple stale review claims",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, time.Now().UTC())
					expiredTime := time.Now().UTC().Add(-1 * time.Hour)
					task.ReviewLeaseExpires = &expiredTime
					return task
				}(),
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReviewing, time.Now().UTC())
					reviewer := "code-reviewer-2"
					task.ReviewingBy = &reviewer
					expiredTime := time.Now().UTC().Add(-2 * time.Hour)
					task.ReviewLeaseExpires = &expiredTime
					return task
				}(),
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-3", models.TaskStatusReviewing, time.Now().UTC())
					expiredTime := time.Now().UTC().Add(-30 * time.Minute)
					task.ReviewLeaseExpires = &expiredTime
					return task
				}(),
			},
			wantCleared: 3,
			wantErr:     false,
		},
		{
			name: "no stale claims (all valid)",
			tasks: []models.Task{
				// Task in REVIEWING with valid (future) lease
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, time.Now().UTC())
					futureTime := time.Now().UTC().Add(30 * time.Minute)
					task.ReviewLeaseExpires = &futureTime
					return task
				}(),
			},
			wantCleared: 0,
			wantErr:     false,
		},
		{
			name: "mixed stale and valid claims",
			tasks: []models.Task{
				// Stale claim
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, time.Now().UTC())
					expiredTime := time.Now().UTC().Add(-1 * time.Hour)
					task.ReviewLeaseExpires = &expiredTime
					return task
				}(),
				// Valid claim
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReviewing, time.Now().UTC())
					futureTime := time.Now().UTC().Add(30 * time.Minute)
					task.ReviewLeaseExpires = &futureTime
					return task
				}(),
				// Stale claim
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-3", models.TaskStatusReviewing, time.Now().UTC())
					reviewer := "code-reviewer-3"
					task.ReviewingBy = &reviewer
					expiredTime := time.Now().UTC().Add(-2 * time.Hour)
					task.ReviewLeaseExpires = &expiredTime
					return task
				}(),
			},
			wantCleared: 2,
			wantErr:     false,
			validateFunc: func(t *testing.T, state *models.State, entries []log.Entry) {
				// task-1 should be cleared and reverted to READY_FOR_REVIEW
				if state.Tasks[0].ReviewingBy != nil {
					t.Errorf("task-1 ReviewingBy should be nil")
				}
				if state.Tasks[0].Status != models.TaskStatusReadyForReview {
					t.Errorf("task-1 should be READY_FOR_REVIEW, got %s", state.Tasks[0].Status)
				}
				// task-2 should NOT be cleared (still REVIEWING)
				if state.Tasks[1].ReviewingBy == nil {
					t.Errorf("task-2 ReviewingBy should not be nil")
				}
				if state.Tasks[1].Status != models.TaskStatusReviewing {
					t.Errorf("task-2 should still be REVIEWING, got %s", state.Tasks[1].Status)
				}
				// task-3 should be cleared and reverted to READY_FOR_REVIEW
				if state.Tasks[2].ReviewingBy != nil {
					t.Errorf("task-3 ReviewingBy should be nil")
				}
				if state.Tasks[2].Status != models.TaskStatusReadyForReview {
					t.Errorf("task-3 should be READY_FOR_REVIEW, got %s", state.Tasks[2].Status)
				}
			},
		},
		{
			name: "no stale claims (no REVIEWING tasks)",
			tasks: []models.Task{
				testhelpers.BuildTaskByStatus("task-1", models.TaskStatusImplementing, time.Now().UTC()),
				testhelpers.BuildTaskByStatus("task-2", models.TaskStatusReady, time.Now().UTC()),
			},
			wantCleared: 0,
			wantErr:     false,
		},
		{
			name: "no stale claims (READY_FOR_REVIEW without reviewing_by)",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReadyForReview, time.Now().UTC())
					return task
				}(),
			},
			wantCleared: 0,
			wantErr:     false,
		},
		{
			name: "stale claim with nil lease (malformed state)",
			tasks: []models.Task{
				func() models.Task {
					task := testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReviewing, time.Now().UTC())
					// Set reviewing_by but nil lease (malformed)
					task.ReviewLeaseExpires = nil
					return task
				}(),
			},
			wantCleared: 1, // Should still clear this malformed state
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp directory
			tmpDir := t.TempDir()
			t.Cleanup(ops.SetAgentProcessProcRootForTest(filepath.Join(t.TempDir(), "missing-proc")))

			// Setup liza directory
			stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)
			testhelpers.SetupPipelineConfig(t, tmpDir)
			logFile := paths.New(tmpDir).LogPath()

			// Create initial state
			initialState := testhelpers.CreateValidState()
			initialState.Tasks = tt.tasks
			addLiveReviewerAgentsForFutureClaims(initialState)

			bb := testhelpers.WriteInitialState(t, stateFile, initialState)

			// Run command
			cleared, err := ClearStaleReviewClaimsCommand(tmpDir)

			// Check error
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				testhelpers.AssertErrorContains(t, err, tt.errContains)
				return
			}

			testhelpers.AssertNoError(t, err)

			// Check cleared count
			if cleared != tt.wantCleared {
				t.Errorf("Cleared = %d, want %d", cleared, tt.wantCleared)
			}

			// Read state to verify changes
			state, err := bb.Read()
			if err != nil {
				t.Fatalf("Failed to read state: %v", err)
			}

			// Read log entries
			logger := log.New(logFile)
			entries, err := logger.Read()
			if err != nil {
				t.Fatalf("Failed to read log: %v", err)
			}

			// Run custom validation if provided
			if tt.validateFunc != nil {
				tt.validateFunc(t, state, entries)
			}
		})
	}
}

func addLiveReviewerAgentsForFutureClaims(state *models.State) {
	now := time.Now().UTC()
	for _, task := range state.Tasks {
		if task.Status != models.TaskStatusReviewing || task.ReviewingBy == nil || task.ReviewLeaseExpires == nil {
			continue
		}
		if !task.ReviewLeaseExpires.After(now) {
			continue
		}
		reviewer := *task.ReviewingBy
		state.Agents[reviewer] = models.Agent{
			Role:        "code-reviewer",
			Status:      models.AgentStatusReviewing,
			CurrentTask: testhelpers.StringPtr(task.ID),
			PID:         os.Getpid(),
		}
	}
}

func TestClearStaleReviewClaimsCommandErrors(t *testing.T) {
	tests := []struct {
		name        string
		setupFunc   func(*testing.T) string
		wantErr     bool
		errContains string
	}{
		{
			name: "state file not found",
			setupFunc: func(t *testing.T) string {
				return "/nonexistent/path"
			},
			wantErr:     true,
			errContains: "failed to load pipeline config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			projectRoot := tt.setupFunc(t)

			_, err := ClearStaleReviewClaimsCommand(projectRoot)

			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error but got none")
					return
				}
				testhelpers.AssertErrorContains(t, err, tt.errContains)
			} else {
				testhelpers.AssertNoError(t, err)
			}
		})
	}
}
