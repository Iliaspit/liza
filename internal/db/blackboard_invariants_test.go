package db

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/embedded"
	"github.com/liza-mas/liza/internal/models"
)

// invariantTestBlackboard sets up a project dir with a frozen pipeline config
// (so the write funnel's classifier loads) and an initial state with one
// DRAFT_CODE coding-pair task.
func invariantTestBlackboard(t *testing.T) *Blackboard {
	t.Helper()
	root := t.TempDir()
	lizaDir := filepath.Join(root, ".liza")
	if err := os.MkdirAll(lizaDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := embedded.WritePipelineConfig(lizaDir, nil); err != nil {
		t.Fatalf("WritePipelineConfig failed: %v", err)
	}

	bb := New(filepath.Join(lizaDir, "state.yaml"))
	if err := bb.Write(&models.State{
		Version: 1,
		Tasks: []models.Task{
			{ID: "task-1", Status: "DRAFT_CODE", RolePair: "coding-pair"},
		},
		Agents: map[string]models.Agent{},
	}); err != nil {
		t.Fatalf("initial Write failed: %v", err)
	}
	return bb
}

func TestWriteFunnel_RejectsInvalidTaskFromNamedOp(t *testing.T) {
	bb := invariantTestBlackboard(t)

	// Claim without worktree/lease/base_commit — structurally invalid.
	err := bb.ModifyOp("claim_task", func(state *models.State) error {
		task := state.FindTask("task-1")
		task.Status = "IMPLEMENTING_CODE"
		agent := "coder-1"
		task.AssignedTo = &agent
		return nil
	})

	var invErr *WriteInvariantError
	if !errors.As(err, &invErr) {
		t.Fatalf("expected WriteInvariantError, got %v", err)
	}
	if invErr.Op != "claim_task" || invErr.TaskID != "task-1" {
		t.Errorf("unexpected error attribution: %+v", invErr)
	}

	// The write must have been aborted: state.yaml untouched.
	state, readErr := bb.Read()
	if readErr != nil {
		t.Fatalf("Read failed: %v", readErr)
	}
	if got := state.FindTask("task-1").Status; got != "DRAFT_CODE" {
		t.Errorf("state mutated despite rejection: status = %s", got)
	}
}

func TestWriteFunnel_AcceptsValidTaskFromNamedOp(t *testing.T) {
	bb := invariantTestBlackboard(t)

	err := bb.ModifyOp("claim_task", func(state *models.State) error {
		task := state.FindTask("task-1")
		task.Status = "IMPLEMENTING_CODE"
		agent := "coder-1"
		worktree := ".worktrees/task-1"
		base := "abc123"
		lease := time.Now().Add(30 * time.Minute)
		task.AssignedTo = &agent
		task.Worktree = &worktree
		task.BaseCommit = &base
		task.LeaseExpires = &lease
		return nil
	})
	if err != nil {
		t.Fatalf("valid claim rejected: %v", err)
	}
}

func TestWriteFunnel_GenericModifyRemainsPermissive(t *testing.T) {
	bb := invariantTestBlackboard(t)

	// The same invalid mutation through unmigrated generic Modify must pass
	// (fail-open during the strangler migration).
	err := bb.Modify(func(state *models.State) error {
		task := state.FindTask("task-1")
		task.Status = "IMPLEMENTING_CODE"
		agent := "coder-1"
		task.AssignedTo = &agent
		return nil
	})
	if err != nil {
		t.Fatalf("generic modify unexpectedly rejected: %v", err)
	}
}

func TestWriteFunnel_UntouchedCorruptTaskDoesNotBlockOtherWrites(t *testing.T) {
	bb := invariantTestBlackboard(t)

	// Introduce pre-existing corruption through the permissive generic path.
	if err := bb.Modify(func(state *models.State) error {
		task := state.FindTask("task-1")
		task.Status = "IMPLEMENTING_CODE" // invalid: no worktree/lease
		agent := "coder-1"
		task.AssignedTo = &agent
		return nil
	}); err != nil {
		t.Fatalf("setup modify failed: %v", err)
	}

	// A named op touching a DIFFERENT task must still be writable, otherwise
	// corruption would wedge the whole system.
	err := bb.ModifyOp("add_task", func(state *models.State) error {
		state.Tasks = append(state.Tasks, models.Task{
			ID: "task-2", Status: "DRAFT_CODE", RolePair: "coding-pair",
		})
		return nil
	})
	if err != nil {
		t.Fatalf("named op blocked by unrelated pre-existing corruption: %v", err)
	}
}

func TestWriteFunnel_TouchingPreExistingInvalidTaskAllowed(t *testing.T) {
	bb := invariantTestBlackboard(t)

	// Pre-existing corruption introduced through the permissive generic path.
	if err := bb.Modify(func(state *models.State) error {
		task := state.FindTask("task-1")
		task.Status = "IMPLEMENTING_CODE" // invalid: no worktree/lease
		agent := "coder-1"
		task.AssignedTo = &agent
		return nil
	}); err != nil {
		t.Fatalf("setup modify failed: %v", err)
	}

	// A named op touching the already-invalid task without introducing the
	// violation (no-worse-than-before) must be allowed — e.g. a handoff
	// appending an event.
	err := bb.ModifyOp("handoff", func(state *models.State) error {
		task := state.FindTask("task-1")
		task.HandoffPending = true
		return nil
	})
	if err != nil {
		t.Fatalf("op touching pre-existing-invalid task rejected: %v", err)
	}
}

func TestWriteFunnel_SkipsWithoutPipelineConfig(t *testing.T) {
	dir := t.TempDir()
	bb := New(filepath.Join(dir, "state.yaml"))
	if err := bb.Write(&models.State{
		Version: 1,
		Tasks:   []models.Task{{ID: "task-1", Status: "DRAFT_CODE"}},
		Agents:  map[string]models.Agent{},
	}); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	// No frozen pipeline config → enforcement skipped even for named ops.
	err := bb.ModifyOp("claim_task", func(state *models.State) error {
		state.FindTask("task-1").Status = "IMPLEMENTING_CODE"
		return nil
	})
	if err != nil {
		t.Fatalf("expected fail-open without pipeline config, got: %v", err)
	}
}
