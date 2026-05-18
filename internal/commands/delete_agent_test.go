package commands

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/ops"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// TestIsAgentProcessAlive tests the process liveness check
func TestIsAgentProcessAlive(t *testing.T) {
	tests := []struct {
		name      string
		pid       int
		wantAlive bool
	}{
		{
			name:      "current process is alive",
			pid:       os.Getpid(),
			wantAlive: true,
		},
		{
			name:      "PID 999999 is not alive",
			pid:       999999,
			wantAlive: false,
		},
		{
			name:      "PID 0 is not alive",
			pid:       0,
			wantAlive: false,
		},
		{
			name:      "negative PID is not alive",
			pid:       -1,
			wantAlive: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alive := ops.IsProcessAlive(tt.pid)
			if alive != tt.wantAlive {
				t.Errorf("ops.IsProcessAlive(%d) = %v, want %v", tt.pid, alive, tt.wantAlive)
			}
		})
	}
}

func TestDeleteAgentCommand(t *testing.T) {
	tests := []struct {
		name          string
		agentID       string
		force         bool
		reason        string
		setupState    func(*models.State)
		wantErr       bool
		wantErrMsg    string
		validateState func(*testing.T, *models.State)
	}{
		{
			name:    "successfully delete IDLE agent with no lease",
			agentID: "test-agent-1",
			force:   false,
			reason:  "test deletion",
			setupState: func(state *models.State) {
				state.Agents["test-agent-1"] = models.Agent{
					Role:      "coder",
					Status:    models.AgentStatusIdle,
					Heartbeat: time.Now().UTC(),
					Terminal:  "terminal-1",
				}
			},
			wantErr: false,
			validateState: func(t *testing.T, state *models.State) {
				if _, exists := state.Agents["test-agent-1"]; exists {
					t.Error("Agent should have been deleted")
				}
				// Check HumanNote was added
				if len(state.HumanNotes) == 0 {
					t.Error("Expected HumanNote to be added")
				} else {
					note := state.HumanNotes[len(state.HumanNotes)-1]
					if note.For != "test-agent-1" {
						t.Errorf("Expected HumanNote.For = 'test-agent-1', got %s", note.For)
					}
				}
			},
		},
		{
			name:    "successfully delete agent with expired lease",
			agentID: "test-agent-2",
			force:   false,
			reason:  "expired lease",
			setupState: func(state *models.State) {
				expiredTime := time.Now().UTC().Add(-1 * time.Hour)
				state.Agents["test-agent-2"] = models.Agent{
					Role:         "coder",
					Status:       models.AgentStatusIdle,
					LeaseExpires: &expiredTime,
					Heartbeat:    time.Now().UTC().Add(-2 * time.Hour),
					Terminal:     "terminal-2",
				}
			},
			wantErr: false,
			validateState: func(t *testing.T, state *models.State) {
				if _, exists := state.Agents["test-agent-2"]; exists {
					t.Error("Agent should have been deleted")
				}
			},
		},
		{
			name:    "error when deleting agent with valid lease without force",
			agentID: "test-agent-3",
			force:   false,
			reason:  "test",
			setupState: func(state *models.State) {
				validTime := time.Now().UTC().Add(1 * time.Hour)
				state.Agents["test-agent-3"] = models.Agent{
					Role:         "coder",
					Status:       models.AgentStatusWorking,
					LeaseExpires: &validTime,
					Heartbeat:    time.Now().UTC(),
					Terminal:     "terminal-3",
				}
			},
			wantErr:    true,
			wantErrMsg: "has active lease",
		},
		{
			name:    "force delete agent with valid lease",
			agentID: "test-agent-4",
			force:   true,
			reason:  "forced deletion",
			setupState: func(state *models.State) {
				validTime := time.Now().UTC().Add(1 * time.Hour)
				state.Agents["test-agent-4"] = models.Agent{
					Role:         "coder",
					Status:       models.AgentStatusWorking,
					LeaseExpires: &validTime,
					Heartbeat:    time.Now().UTC(),
					Terminal:     "terminal-4",
				}
			},
			wantErr: false,
			validateState: func(t *testing.T, state *models.State) {
				if _, exists := state.Agents["test-agent-4"]; exists {
					t.Error("Agent should have been deleted with force")
				}
			},
		},
		{
			name:    "error when deleting agent with CurrentTask without force",
			agentID: "test-agent-5",
			force:   false,
			reason:  "test",
			setupState: func(state *models.State) {
				currentTask := "task-1"
				state.Agents["test-agent-5"] = models.Agent{
					Role:        "coder",
					Status:      models.AgentStatusWorking,
					CurrentTask: &currentTask,
					Heartbeat:   time.Now().UTC(),
					Terminal:    "terminal-5",
				}
			},
			wantErr:    true,
			wantErrMsg: "is working on task",
		},
		{
			name:    "force delete agent with CurrentTask",
			agentID: "test-agent-6",
			force:   true,
			reason:  "forced deletion",
			setupState: func(state *models.State) {
				currentTask := "task-1"
				state.Agents["test-agent-6"] = models.Agent{
					Role:        "coder",
					Status:      models.AgentStatusWorking,
					CurrentTask: &currentTask,
					Heartbeat:   time.Now().UTC(),
					Terminal:    "terminal-6",
				}
			},
			wantErr: false,
			validateState: func(t *testing.T, state *models.State) {
				if _, exists := state.Agents["test-agent-6"]; exists {
					t.Error("Agent should have been deleted with force")
				}
			},
		},
		{
			name:       "error on empty agent ID",
			agentID:    "",
			force:      false,
			reason:     "test",
			setupState: func(state *models.State) {},
			wantErr:    true,
			wantErrMsg: "agent ID required",
		},
		{
			name:       "error on nonexistent agent",
			agentID:    "nonexistent-agent",
			force:      false,
			reason:     "test",
			setupState: func(state *models.State) {},
			wantErr:    true,
			wantErrMsg: "agent not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup temp directory
			tmpDir := t.TempDir()

			// Create .liza directory
			lizaDir := paths.New(tmpDir).LizaDir()
			if err := os.MkdirAll(lizaDir, 0755); err != nil {
				t.Fatal(err)
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

			// Execute command (nil stdin means no interactive prompts in these tests)
			err := DeleteAgentCommand(tmpDir, tt.agentID, tt.force, tt.reason, nil)

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
			if tt.validateState != nil {
				finalState, err := bb.Read()
				if err != nil {
					t.Fatal(err)
				}
				tt.validateState(t, finalState)
			}
		})
	}
}

// TestDeleteAgentWithPID tests deletion of agents with PID tracking
func TestDeleteAgentWithPID(t *testing.T) {
	tests := []struct {
		name          string
		agentID       string
		force         bool
		reason        string
		pid           int
		wantErr       bool
		wantErrMsg    string
		validateState func(*testing.T, *models.State)
	}{
		{
			name:    "delete agent with PID 0 (backward compat)",
			agentID: "test-agent-1",
			force:   false,
			reason:  "test deletion",
			pid:     0,
			wantErr: false,
			validateState: func(t *testing.T, state *models.State) {
				if _, exists := state.Agents["test-agent-1"]; exists {
					t.Error("Agent should have been deleted")
				}
			},
		},
		{
			name:    "delete agent with dead process (PID 999999)",
			agentID: "test-agent-2",
			force:   false,
			reason:  "test deletion",
			pid:     999999,
			wantErr: false,
			validateState: func(t *testing.T, state *models.State) {
				if _, exists := state.Agents["test-agent-2"]; exists {
					t.Error("Agent should have been deleted")
				}
			},
		},
		{
			name:    "force delete agent with running process",
			agentID: "test-agent-3",
			force:   true,
			reason:  "forced deletion",
			pid:     os.Getpid(),
			wantErr: false,
			validateState: func(t *testing.T, state *models.State) {
				if _, exists := state.Agents["test-agent-3"]; exists {
					t.Error("Agent should have been deleted with force")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup temp directory
			tmpDir := t.TempDir()

			// Create .liza directory
			lizaDir := paths.New(tmpDir).LizaDir()
			if err := os.MkdirAll(lizaDir, 0755); err != nil {
				t.Fatal(err)
			}

			// Create state file
			statePath := filepath.Join(lizaDir, paths.StateFileName)
			state := testhelpers.CreateValidState()

			// Setup state with agent that has PID
			state.Agents[tt.agentID] = models.Agent{
				Role:      "coder",
				Status:    models.AgentStatusIdle,
				Heartbeat: time.Now().UTC(),
				Terminal:  "terminal-1",
				PID:       tt.pid,
			}

			// Write initial state
			bb := db.New(statePath)
			if err := bb.Write(state); err != nil {
				t.Fatal(err)
			}

			// Execute command (nil stdin means no interactive prompts in these tests)
			err := DeleteAgentCommand(tmpDir, tt.agentID, tt.force, tt.reason, nil)

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
			if tt.validateState != nil {
				finalState, err := bb.Read()
				if err != nil {
					t.Fatal(err)
				}
				tt.validateState(t, finalState)
			}
		})
	}
}

// TestDeleteAgentCommand_InteractivePrompt verifies the PID confirmation prompt
// works correctly with injected stdin (testing the io.Reader injection pattern).
func TestDeleteAgentCommand_InteractivePrompt(t *testing.T) {
	tmpDir := t.TempDir()
	lizaDir := paths.New(tmpDir).LizaDir()
	if err := os.MkdirAll(lizaDir, 0755); err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(lizaDir, paths.StateFileName)

	t.Run("decline_deletion", func(t *testing.T) {
		state := testhelpers.CreateValidState()
		state.Agents["running-agent"] = models.Agent{
			Role:      "coder",
			Status:    models.AgentStatusIdle,
			Heartbeat: time.Now().UTC(),
			Terminal:  "terminal-1",
			PID:       os.Getpid(),
		}
		bb := db.New(statePath)
		if err := bb.Write(state); err != nil {
			t.Fatal(err)
		}
		stdin := strings.NewReader("n\n")
		err := DeleteAgentCommand(tmpDir, "running-agent", false, "test", stdin)
		if err == nil {
			t.Error("Expected error when declining deletion")
		}
		if !strings.Contains(err.Error(), "cancelled") {
			t.Errorf("Expected cancelled in error, got: %v", err)
		}
		finalState, _ := bb.Read()
		if _, exists := finalState.Agents["running-agent"]; !exists {
			t.Error("Agent should NOT have been deleted when user declined")
		}
	})

	t.Run("confirm_deletion", func(t *testing.T) {
		state := testhelpers.CreateValidState()
		state.Agents["running-agent"] = models.Agent{
			Role:      "coder",
			Status:    models.AgentStatusIdle,
			Heartbeat: time.Now().UTC(),
			Terminal:  "terminal-1",
			PID:       os.Getpid(),
		}
		bb := db.New(statePath)
		if err := bb.Write(state); err != nil {
			t.Fatal(err)
		}
		stdin := strings.NewReader("y\n")
		err := DeleteAgentCommand(tmpDir, "running-agent", false, "test", stdin)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		finalState, _ := bb.Read()
		if _, exists := finalState.Agents["running-agent"]; exists {
			t.Error("Agent should have been deleted when user confirmed")
		}
	})
}

func TestDeleteAgentCommand_ReportsTerminationFailure(t *testing.T) {
	original := terminateAgent
	terminateAgent = func(projectRoot, agentID string, force, allowRunningPID bool, reason string, grace time.Duration) (*ops.TerminateAgentResult, error) {
		return nil, errors.New("process still running")
	}
	t.Cleanup(func() { terminateAgent = original })

	err := DeleteAgentCommand(t.TempDir(), "coder-1", true, "test", nil)
	if err == nil {
		t.Fatal("DeleteAgentCommand() error = nil, want process failure")
	}
	if !strings.Contains(err.Error(), "process still running") {
		t.Fatalf("error = %q, want process failure detail", err.Error())
	}
}
