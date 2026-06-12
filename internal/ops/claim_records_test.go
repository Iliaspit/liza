package ops

import (
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
	"github.com/liza-mas/liza/internal/roles"
	"github.com/liza-mas/liza/internal/testhelpers"
)

// TestClaimReleaseCycle_KeepsClaimRecordsInSyncWithLegacyFields drives a full
// doer claim -> release cycle through the real ClaimTask/ReleaseClaim ops and
// asserts the first-class claim records (state.Claims) stay consistent with
// the legacy AssignedTo/LeaseExpires fields at every step (dual-write).
func TestClaimReleaseCycle_KeepsClaimRecordsInSyncWithLegacyFields(t *testing.T) {
	tmpDir := t.TempDir()
	testhelpers.SetupTestGitRepo(t, tmpDir)
	stateFile, _ := testhelpers.SetupLizaDir(t, tmpDir)

	now := time.Now().UTC()
	state := testhelpers.CreateValidState()
	registerClaimTaskTestAgents(state)
	state.Tasks = []models.Task{
		testhelpers.BuildTaskByStatus("task-1", models.TaskStatusReady, now),
	}
	testhelpers.WriteInitialState(t, stateFile, state)

	// Claim: a doer claim record must mirror the legacy fields.
	if _, err := ClaimTask(tmpDir, "task-1", "coder-1"); err != nil {
		t.Fatalf("ClaimTask() error: %v", err)
	}

	claimed := readClaimStateForTest(t, stateFile)
	task := claimed.FindTask("task-1")
	if task == nil {
		t.Fatal("task not found after claim")
	}
	if task.AssignedTo == nil || *task.AssignedTo != "coder-1" {
		t.Fatal("legacy AssignedTo not set — test precondition broken")
	}
	claim := claimed.FindClaim("task-1", models.ClaimKindDoer)
	if claim == nil {
		t.Fatal("doer claim record missing after ClaimTask")
	}
	if claim.AgentID != *task.AssignedTo {
		t.Errorf("claim AgentID = %q, want legacy AssignedTo %q", claim.AgentID, *task.AssignedTo)
	}
	if task.LeaseExpires == nil {
		t.Fatal("legacy LeaseExpires not set after claim")
	}
	if claim.ExpiresAt == nil || !claim.ExpiresAt.Equal(*task.LeaseExpires) {
		t.Errorf("claim ExpiresAt = %v, want legacy LeaseExpires %v", claim.ExpiresAt, task.LeaseExpires)
	}
	if claim.GrantedAt.IsZero() {
		t.Error("claim GrantedAt is zero")
	}
	if got := claimed.ClaimsForAgent("coder-1"); len(got) != 1 {
		t.Errorf("ClaimsForAgent(coder-1) = %d claims, want 1", len(got))
	}

	// Release: the claim record must be removed alongside the legacy fields.
	result, err := ReleaseClaim(tmpDir, "task-1", roles.ClaimDoer, true, "test cycle", "human")
	if err != nil {
		t.Fatalf("ReleaseClaim() error: %v", err)
	}
	if !result.ReleasedDoer {
		t.Fatal("ReleaseClaim did not release doer claim")
	}

	released := readClaimStateForTest(t, stateFile)
	task = released.FindTask("task-1")
	if task.AssignedTo != nil || task.LeaseExpires != nil {
		t.Fatalf("legacy fields not cleared: AssignedTo=%v LeaseExpires=%v", task.AssignedTo, task.LeaseExpires)
	}
	if c := released.FindClaim("task-1", models.ClaimKindDoer); c != nil {
		t.Errorf("doer claim record still present after release: %+v", c)
	}
	if got := released.ClaimsForAgent("coder-1"); len(got) != 0 {
		t.Errorf("ClaimsForAgent(coder-1) after release = %+v, want none", got)
	}
}

func TestSweepExpiredClaims(t *testing.T) {
	now := time.Now().UTC()
	longExpired := now.Add(-models.LeaseExpiryGracePeriod - time.Minute)
	withinGrace := now.Add(-models.LeaseExpiryGracePeriod + time.Minute)
	live := now.Add(time.Hour)

	state := &models.State{
		Claims: []models.Claim{
			{TaskID: "task-1", AgentID: "coder-1", Kind: models.ClaimKindDoer, ExpiresAt: &longExpired},
			{TaskID: "task-2", AgentID: "coder-2", Kind: models.ClaimKindDoer, ExpiresAt: &withinGrace},
			{TaskID: "task-3", AgentID: "reviewer-1", Kind: models.ClaimKindReviewer, ExpiresAt: &live},
			{TaskID: "task-4", AgentID: "coder-3", Kind: models.ClaimKindDoer}, // no lease — never swept
		},
	}

	expired := SweepExpiredClaims(state, now)
	if len(expired) != 1 {
		t.Fatalf("SweepExpiredClaims() = %d claims, want 1: %+v", len(expired), expired)
	}
	if expired[0].TaskID != "task-1" {
		t.Errorf("swept TaskID = %q, want task-1", expired[0].TaskID)
	}
	if len(state.Claims) != 4 {
		t.Errorf("SweepExpiredClaims must not mutate state, len(Claims) = %d", len(state.Claims))
	}
}
