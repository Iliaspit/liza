package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/brand"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/paths"
	"github.com/liza-mas/liza/internal/testhelpers"
)

func TestBuildRespawnArgs(t *testing.T) {
	tests := []struct {
		name    string
		role    string
		agentID string
		cli     string
		want    []string
	}{
		{
			name:    "coder respawn",
			role:    "coder",
			agentID: "coder-1",
			cli:     "claude",
			want:    []string{brand.BinaryName, "agent", "coder", "--agent-id", "coder-1", "--cli", "claude"},
		},
		{
			name:    "code-reviewer respawn uses hyphenated role",
			role:    "code-reviewer",
			agentID: "code-reviewer-1",
			cli:     "claude",
			want:    []string{brand.BinaryName, "agent", "code-reviewer", "--agent-id", "code-reviewer-1", "--cli", "claude"},
		},
		{
			name:    "orchestrator respawn",
			role:    "orchestrator",
			agentID: "orchestrator-1",
			cli:     "codex",
			want:    []string{brand.BinaryName, "agent", "orchestrator", "--agent-id", "orchestrator-1", "--cli", "codex"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildRespawnArgs(tt.role, tt.agentID, tt.cli)
			if len(got) != len(tt.want) {
				t.Fatalf("buildRespawnArgs() = %v, want %v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("arg[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestRecoverAgentCommand(t *testing.T) {
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
			name:    "recover idle agent with no task",
			agentID: "coder-1",
			force:   false,
			reason:  "crashed",
			setupState: func(state *models.State) {
				state.Agents["coder-1"] = models.Agent{
					Role:      "coder",
					Status:    models.AgentStatusIdle,
					Heartbeat: time.Now().UTC(),
				}
			},
			wantErr: false,
			validateState: func(t *testing.T, state *models.State) {
				if _, exists := state.Agents["coder-1"]; exists {
					t.Error("Agent should have been deleted")
				}
				if len(state.HumanNotes) == 0 {
					t.Error("Expected HumanNote to be added")
				} else {
					note := state.HumanNotes[len(state.HumanNotes)-1]
					if note.For != "coder-1" {
						t.Errorf("Expected HumanNote.For = 'coder-1', got %s", note.For)
					}
				}
			},
		},
		{
			name:       "recover nonexistent agent is idempotent",
			agentID:    "nonexistent",
			force:      false,
			reason:     "cleanup",
			setupState: func(state *models.State) {},
			wantErr:    false,
			validateState: func(t *testing.T, state *models.State) {
				// No changes expected
			},
		},
		{
			name:    "recover reviewer with reviewing task",
			agentID: "code-reviewer-1",
			force:   false,
			reason:  "OOM",
			setupState: func(state *models.State) {
				expiredTime := time.Now().UTC().Add(-1 * time.Hour)
				state.Agents["code-reviewer-1"] = models.Agent{
					Role:         "code-reviewer",
					Status:       models.AgentStatusReviewing,
					CurrentTask:  strPtr("task-1"),
					LeaseExpires: &expiredTime,
					Heartbeat:    time.Now().UTC(),
				}
				state.Tasks = append(state.Tasks, models.Task{
					ID:                 "task-1",
					Description:        "test task",
					Status:             models.TaskStatusReviewing,
					RolePair:           "coding-pair",
					Priority:           1,
					ReviewingBy:        strPtr("code-reviewer-1"),
					ReviewLeaseExpires: &expiredTime,
					SpecRef:            "spec.md",
					DoneWhen:           "tests pass",
					Scope:              "small",
				})
			},
			wantErr: false,
			validateState: func(t *testing.T, state *models.State) {
				if _, exists := state.Agents["code-reviewer-1"]; exists {
					t.Error("Agent should have been deleted")
				}
				task := state.FindTask("task-1")
				if task == nil {
					t.Fatal("Task should still exist")
				}
				if task.Status != models.TaskStatusReadyForReview {
					t.Errorf("Task status = %s, want %s", task.Status, models.TaskStatusReadyForReview)
				}
				if task.ReviewingBy != nil {
					t.Error("Task ReviewingBy should be nil")
				}
			},
		},
		{
			name:    "error on alive PID without force",
			agentID: "coder-1",
			force:   false,
			reason:  "test",
			setupState: func(state *models.State) {
				state.Agents["coder-1"] = models.Agent{
					Role:   "coder",
					Status: models.AgentStatusWorking,
					PID:    os.Getpid(),
				}
			},
			wantErr:    true,
			wantErrMsg: "still running",
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			lizaDir := paths.New(tmpDir).LizaDir()
			if err := os.MkdirAll(lizaDir, 0755); err != nil {
				t.Fatal(err)
			}
			testhelpers.SetupPipelineConfig(t, tmpDir)

			statePath := filepath.Join(lizaDir, paths.StateFileName)
			state := testhelpers.CreateValidState()
			tt.setupState(state)

			bb := db.New(statePath)
			if err := bb.Write(state); err != nil {
				t.Fatal(err)
			}

			err := RecoverAgentCommand(tmpDir, tt.agentID, tt.force, "", tt.reason)

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
