package journal

import (
	"testing"

	"github.com/liza-mas/liza/internal/models"
)

func TestReconstruct_ClaimsFold(t *testing.T) {
	events := []Event{
		{Type: EventClaimGranted, Task: "t1", Agent: "coder-1", Fields: map[string]any{"kind": "doer"}},
		{Type: EventClaimGranted, Task: "t1", Agent: "code-reviewer-1", Fields: map[string]any{"kind": "reviewer"}},
		{Type: EventClaimReleased, Task: "t1", Agent: "coder-1", Fields: map[string]any{"kind": "doer"}},
		{Type: EventClaimGranted, Task: "t2", Agent: "coder-2", Fields: map[string]any{"kind": "doer"}},
	}
	r := Reconstruct(events)

	if _, ok := r.Claims[ClaimKey{"t1", "doer"}]; ok {
		t.Error("released doer claim on t1 should be gone")
	}
	if r.Claims[ClaimKey{"t1", "reviewer"}] != "code-reviewer-1" {
		t.Errorf("t1 reviewer claim = %q, want code-reviewer-1", r.Claims[ClaimKey{"t1", "reviewer"}])
	}
	if r.Claims[ClaimKey{"t2", "doer"}] != "coder-2" {
		t.Errorf("t2 doer claim = %q, want coder-2", r.Claims[ClaimKey{"t2", "doer"}])
	}
}

func TestReconstruct_DiffClaimsAgainstState(t *testing.T) {
	events := []Event{
		{Type: EventClaimGranted, Task: "t1", Agent: "coder-1", Fields: map[string]any{"kind": "doer"}},
	}
	r := Reconstruct(events)

	// Coherent: state matches journal.
	coherent := &models.State{Claims: []models.Claim{{TaskID: "t1", AgentID: "coder-1", Kind: "doer"}}}
	if d := r.DiffClaims(coherent); len(d) != 0 {
		t.Errorf("expected no diff, got %v", d)
	}

	// Holder mismatch.
	mismatch := &models.State{Claims: []models.Claim{{TaskID: "t1", AgentID: "coder-9", Kind: "doer"}}}
	if d := r.DiffClaims(mismatch); len(d) != 1 {
		t.Errorf("expected 1 holder-mismatch diff, got %v", d)
	}

	// Journal has a claim state doesn't.
	empty := &models.State{}
	if d := r.DiffClaims(empty); len(d) != 1 {
		t.Errorf("expected 1 journal-only diff, got %v", d)
	}
}

func TestReconstruct_SingletonsFoldAndDiff(t *testing.T) {
	events := []Event{
		{Type: EventSprintAdvanced, Fields: map[string]any{"to": float64(2)}}, // JSON round-trip
		{Type: EventSprintStatus, Fields: map[string]any{"to": "IN_PROGRESS"}},
		{Type: EventCircuitBreaker, Fields: map[string]any{"to": "OK"}},
		{Type: EventGoalStatus, Fields: map[string]any{"to": "IN_PROGRESS"}},
		{Type: EventSystemMode, Fields: map[string]any{"to": "PAUSED"}},
	}
	r := Reconstruct(events)
	if r.SprintNumber != 2 || !r.sprintSeen {
		t.Errorf("sprint number = %d seen=%v, want 2/true", r.SprintNumber, r.sprintSeen)
	}

	coherent := &models.State{}
	coherent.Sprint.Number = 2
	coherent.Sprint.Status = "IN_PROGRESS"
	coherent.CircuitBreaker.Status = "OK"
	coherent.Goal.Status = "IN_PROGRESS"
	coherent.Config.Mode = "PAUSED"
	if d := r.DiffSingletons(coherent); len(d) != 0 {
		t.Errorf("expected no singleton diff, got %v", d)
	}

	drift := &models.State{}
	drift.Sprint.Number = 3
	drift.Sprint.Status = "CHECKPOINT"
	drift.CircuitBreaker.Status = "TRIGGERED"
	drift.Goal.Status = "COMPLETED"
	drift.Config.Mode = "RUNNING"
	if d := r.DiffSingletons(drift); len(d) != 5 {
		t.Errorf("expected 5 singleton diffs, got %d: %v", len(d), d)
	}
}

func TestReconstruct_UnseenSingletonsDoNotWarn(t *testing.T) {
	// Empty journal has no opinion on any singleton — a pre-journal project
	// with populated state must not produce divergence noise.
	r := Reconstruct(nil)
	state := &models.State{}
	state.Sprint.Number = 5
	state.Sprint.Status = "IN_PROGRESS"
	state.CircuitBreaker.Status = "OK"
	state.Goal.Status = "IN_PROGRESS"
	state.Config.Mode = "RUNNING"
	if d := r.DiffSingletons(state); len(d) != 0 {
		t.Errorf("unseen singletons should not warn, got %v", d)
	}
	if d := r.DiffClaims(&models.State{Claims: []models.Claim{{TaskID: "t", AgentID: "a", Kind: "doer"}}}); len(d) != 1 {
		t.Errorf("a state claim with no journal record IS a divergence, got %v", d)
	}
}
