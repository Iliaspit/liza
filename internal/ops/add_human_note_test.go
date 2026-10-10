package ops

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/db"
	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/statehygiene"
	"github.com/liza-mas/liza/internal/statevalidate"
	"github.com/liza-mas/liza/internal/testhelpers"
	"gopkg.in/yaml.v3"
)

func setupHumanNoteState(t *testing.T) (*db.Blackboard, string) {
	t.Helper()
	root := t.TempDir()
	statePath, _ := testhelpers.SetupLizaDir(t, root)
	testhelpers.SetupPipelineConfig(t, root)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# Human note fixture\n"), 0644); err != nil {
		t.Fatal(err)
	}
	state := testhelpers.CreateValidState()
	state.Goal.SpecRef = "README.md"
	now := time.Now().UTC().Add(-time.Hour)
	task := testhelpers.BuildTaskByStatus("planning-1", models.TaskStatusBlocked, now)
	task.Type, task.RolePair = models.TaskTypePlanning, "code-planning-pair"
	owner, worktree, base := "code-planner-1", ".worktrees/planning-1", "original-base"
	lease := now.Add(2 * time.Hour)
	task.AssignedTo, task.Worktree, task.BaseCommit, task.LeaseExpires = &owner, &worktree, &base, &lease
	task.Attempt = 2
	task.History = []models.TaskHistoryEntry{{Time: now, Event: models.TaskEventOrchestratorAssessment}}
	task.Extra = map[string]any{"draft_marker": "preserve"}
	state.Tasks = []models.Task{task, testhelpers.BuildTaskByStatus("other-1", models.TaskStatusReady, now)}
	state.Sprint.Scope.Planned = []string{"planning-1", "other-1"}
	state.Agents[owner] = models.Agent{Role: models.RoleCodePlanner, Status: models.AgentStatusHandoff, Generation: "original-generation", CurrentTask: &task.ID, LeaseExpires: &lease, Heartbeat: now, Provider: "codex", PID: os.Getpid(), Extra: map[string]any{"marker": "keep"}}
	state.HumanNotes = []models.HumanNote{{Timestamp: now, Message: "existing", For: task.ID, Extra: map[string]any{"changed_by": "human"}}}
	state.Extra = map[string]any{"unrelated_metadata": "unchanged"}
	if err := statevalidate.ValidateState(state, root, false, io.Discard); err != nil {
		t.Fatalf("invalid canonical fixture: %v", err)
	}
	return testhelpers.WriteInitialState(t, statePath, state), statePath
}

func TestAddHumanNoteRejectsNoncanonicalStateWithoutMutation(t *testing.T) {
	tests := []struct {
		name     string
		mutate   func(*models.State)
		contains string
	}{
		{"duplicate task IDs", func(state *models.State) { state.Tasks = append(state.Tasks, state.Tasks[0]) }, "duplicate task ID"},
		{"invalid lifecycle", func(state *models.State) { state.FindTask("other-1").Status = models.TaskStatusImplementing }, "without assigned_to"},
		{"legacy underscore role", func(state *models.State) {
			agent := state.Agents["code-planner-1"]
			agent.Role = "code_planner"
			state.Agents["code-planner-1"] = agent
		}, "requires normalization"},
		{"legacy attempted field", func(state *models.State) {
			task := state.FindTask("planning-1")
			task.Attempt = 0
			task.Extra["attempted"] = []any{"previous-planner"}
		}, "requires normalization"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bb, path := setupHumanNoteState(t)
			state, err := bb.Read()
			if err != nil {
				t.Fatal(err)
			}
			state.HumanNotes = []models.HumanNote{}
			tt.mutate(state)
			// Write the adversarial fixture directly so native db normalization
			// cannot repair it before AddHumanNote encounters the locked bytes.
			data, err := yaml.Marshal(state)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, data, 0644); err != nil {
				t.Fatal(err)
			}
			before := readHumanNoteBytes(t, path)
			err = AddHumanNote(bb, "planning-1", "answer")
			if err == nil || !strings.Contains(err.Error(), tt.contains) {
				t.Fatalf("error = %v, want %q", err, tt.contains)
			}
			after := readHumanNoteBytes(t, path)
			if !bytes.Equal(before, after) {
				t.Fatal("noncanonical rejection changed persisted bytes")
			}
			var raw models.State
			if err := yaml.Unmarshal(after, &raw); err != nil {
				t.Fatal(err)
			}
			if len(raw.HumanNotes) != 0 {
				t.Fatal("noncanonical rejection appended a note")
			}
		})
	}
}

func TestAddHumanNotePreservesCanonicalState(t *testing.T) {
	bb, statePath := setupHumanNoteState(t)
	before, err := bb.Read()
	if err != nil {
		t.Fatal(err)
	}
	message := " \n  Answer: λ is supported.\nKeep the staged draft.  \n"
	started := time.Now().UTC()
	for i := 0; i < 2; i++ {
		if err := AddHumanNote(bb, "planning-1", message); err != nil {
			t.Fatal(err)
		}
	}
	finished := time.Now().UTC()
	// A fresh instance proves the operation persisted before reporting success.
	after, err := db.New(statePath).Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(after.HumanNotes) != len(before.HumanNotes)+2 {
		t.Fatalf("notes = %d, want %d", len(after.HumanNotes), len(before.HumanNotes)+2)
	}
	for _, note := range after.HumanNotes[len(before.HumanNotes):] {
		if note.Message != message || note.For != "planning-1" || !reflect.DeepEqual(note.Extra, map[string]any{"changed_by": "human"}) {
			t.Fatalf("unexpected note: %#v", note)
		}
		if note.Timestamp.IsZero() || note.Timestamp.Before(started) || note.Timestamp.After(finished) || note.Timestamp.Location() != time.UTC {
			t.Fatalf("timestamp %v outside UTC append interval [%v, %v]", note.Timestamp, started, finished)
		}
	}
	after.HumanNotes = after.HumanNotes[:len(before.HumanNotes)]
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("append changed other canonical state\nbefore: %#v\nafter: %#v", before, after)
	}
}

func TestAddHumanNoteRejectsInvalidInputsWithoutMutation(t *testing.T) {
	tests := []struct{ name, taskID, message string }{
		{"empty ID", "", "answer"}, {"traversal", "../planning-1", "answer"},
		{"spaces in ID", " planning-1", "answer"}, {"unknown", "missing-1", "answer"},
		{"all", "all", "answer"}, {"empty message", "planning-1", ""},
		{"blank message", "planning-1", " \t\n\u2003"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bb, path := setupHumanNoteState(t)
			before := readHumanNoteBytes(t, path)
			if err := AddHumanNote(bb, tt.taskID, tt.message); err == nil {
				t.Fatal("expected rejection")
			}
			if !bytes.Equal(before, readHumanNoteBytes(t, path)) {
				t.Fatal("rejected input mutated state bytes")
			}
		})
	}
}

func TestAddHumanNoteRejectsEveryNonBlockedStatus(t *testing.T) {
	statuses := []models.TaskStatus{
		models.TaskStatusDraft, models.TaskStatusReady, models.TaskStatusImplementing,
		models.TaskStatusReadyForReview, models.TaskStatusLegacyReadyForReview, models.TaskStatusReviewing,
		models.TaskStatusRejected, models.TaskStatusApproved, models.TaskStatusMerged,
		models.TaskStatusAbandoned, models.TaskStatusSuperseded, models.TaskStatusIntegrationFailed,
		models.TaskStatusDraftCodingPlan, models.TaskStatusCodePlanning, models.TaskStatusCodingPlanToReview,
		models.TaskStatusReviewingCodingPlan, models.TaskStatusCodingPlanApproved, models.TaskStatusCodingPlanRejected,
		models.TaskStatusPartiallyApproved, models.TaskStatusReviewingCode2,
		models.TaskStatus("DRAFT_EPIC_PLAN"), models.TaskStatus("EPIC_PLANNING"),
		models.TaskStatus("EPIC_PLAN_TO_REVIEW"), models.TaskStatus("REVIEWING_EPIC_PLAN"),
		models.TaskStatus("EPIC_PLAN_APPROVED"), models.TaskStatus("EPIC_PLAN_REJECTED"),
		models.TaskStatus("DRAFT_US"), models.TaskStatus("WRITING_US"), models.TaskStatus("US_TO_REVIEW"),
		models.TaskStatus("REVIEWING_US"), models.TaskStatus("US_APPROVED"), models.TaskStatus("US_REJECTED"),
		models.TaskStatus("CUSTOM_STATE"), models.TaskStatus(""),
	}
	for _, status := range statuses {
		t.Run(string(status), func(t *testing.T) {
			bb, path := setupHumanNoteState(t)
			if err := bb.Modify(func(state *models.State) error { state.FindTask("planning-1").Status = status; return nil }); err != nil {
				t.Fatal(err)
			}
			before := readHumanNoteBytes(t, path)
			if err := AddHumanNote(bb, "planning-1", "answer"); err == nil {
				t.Fatalf("accepted status %q", status)
			}
			if !bytes.Equal(before, readHumanNoteBytes(t, path)) {
				t.Fatal("rejection changed persisted state")
			}
		})
	}
}

func TestAddHumanNoteRevalidatesLockedState(t *testing.T) {
	for _, remove := range []bool{false, true} {
		t.Run(map[bool]string{false: "status changed", true: "task removed"}[remove], func(t *testing.T) {
			bb, path := setupHumanNoteState(t)
			if _, err := bb.ReadCached(); err != nil {
				t.Fatal(err)
			}
			if err := db.New(path).Modify(func(state *models.State) error {
				if remove {
					state.Tasks = state.Tasks[1:]
				} else {
					state.FindTask("planning-1").Status = models.TaskStatusReady
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			before := readHumanNoteBytes(t, path)
			if err := AddHumanNote(bb, "planning-1", "answer"); err == nil {
				t.Fatal("accepted stale cached BLOCKED target")
			}
			if !bytes.Equal(before, readHumanNoteBytes(t, path)) {
				t.Fatal("rejection mutated state")
			}
		})
	}
}

func TestAddHumanNotePersistenceFailureDoesNotReportSuccess(t *testing.T) {
	bb, path := setupHumanNoteState(t)
	before := readHumanNoteBytes(t, path)
	if err := AddHumanNote(bb, "planning-1", strings.Repeat("x", statehygiene.MaxStateTextBytes+1)); err == nil {
		t.Fatal("expected native persistence hygiene failure")
	}
	if !bytes.Equal(before, readHumanNoteBytes(t, path)) {
		t.Fatal("failed persistence mutated state")
	}
	missing := db.New(filepath.Join(t.TempDir(), "state.yaml"))
	if err := AddHumanNote(missing, "planning-1", "answer"); err == nil {
		t.Fatal("missing state reported success")
	}
}

func readHumanNoteBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
