package models

import (
	"testing"
	"time"
)

func claimTestState() *State {
	exp := time.Date(2026, 6, 12, 12, 0, 0, 0, time.UTC)
	return &State{
		Claims: []Claim{
			{TaskID: "task-1", AgentID: "coder-1", Kind: ClaimKindDoer, GrantedAt: exp.Add(-time.Hour), ExpiresAt: &exp},
			{TaskID: "task-1", AgentID: "reviewer-1", Kind: ClaimKindReviewer, GrantedAt: exp.Add(-time.Hour), ExpiresAt: &exp},
			{TaskID: "task-2", AgentID: "coder-1", Kind: ClaimKindDoer, GrantedAt: exp.Add(-time.Hour)},
		},
	}
}

func TestFindClaim(t *testing.T) {
	s := claimTestState()

	c := s.FindClaim("task-1", ClaimKindDoer)
	if c == nil {
		t.Fatal("FindClaim(task-1, doer) = nil, want claim")
	}
	if c.AgentID != "coder-1" {
		t.Errorf("AgentID = %q, want coder-1", c.AgentID)
	}

	if got := s.FindClaim("task-1", ClaimKindReviewer); got == nil || got.AgentID != "reviewer-1" {
		t.Errorf("FindClaim(task-1, reviewer) = %+v, want reviewer-1", got)
	}
	if got := s.FindClaim("task-3", ClaimKindDoer); got != nil {
		t.Errorf("FindClaim(task-3, doer) = %+v, want nil", got)
	}
	if got := s.FindClaim("task-2", ClaimKindReviewer); got != nil {
		t.Errorf("FindClaim(task-2, reviewer) = %+v, want nil", got)
	}
}

func TestFindClaimReturnsPointerIntoState(t *testing.T) {
	s := claimTestState()

	c := s.FindClaim("task-2", ClaimKindDoer)
	if c == nil {
		t.Fatal("FindClaim(task-2, doer) = nil, want claim")
	}
	newExp := time.Date(2026, 6, 12, 18, 0, 0, 0, time.UTC)
	c.ExpiresAt = &newExp

	again := s.FindClaim("task-2", ClaimKindDoer)
	if again.ExpiresAt == nil || !again.ExpiresAt.Equal(newExp) {
		t.Errorf("mutation through FindClaim pointer not reflected in state: %+v", again)
	}
}

func TestClaimsForAgent(t *testing.T) {
	s := claimTestState()

	claims := s.ClaimsForAgent("coder-1")
	if len(claims) != 2 {
		t.Fatalf("ClaimsForAgent(coder-1) returned %d claims, want 2", len(claims))
	}
	for _, c := range claims {
		if c.AgentID != "coder-1" {
			t.Errorf("claim %+v has wrong agent", c)
		}
	}

	if got := s.ClaimsForAgent("nobody"); len(got) != 0 {
		t.Errorf("ClaimsForAgent(nobody) = %+v, want empty", got)
	}
}

func TestGrantClaimAppendsNewClaim(t *testing.T) {
	s := &State{}
	s.GrantClaim(Claim{TaskID: "task-1", AgentID: "coder-1", Kind: ClaimKindDoer, GrantedAt: time.Now().UTC()})

	if len(s.Claims) != 1 {
		t.Fatalf("len(Claims) = %d, want 1", len(s.Claims))
	}
	if c := s.FindClaim("task-1", ClaimKindDoer); c == nil || c.AgentID != "coder-1" {
		t.Errorf("FindClaim after grant = %+v, want coder-1", c)
	}
}

func TestGrantClaimReplacesSameTaskAndKind(t *testing.T) {
	s := claimTestState()

	granted := time.Date(2026, 6, 12, 13, 0, 0, 0, time.UTC)
	s.GrantClaim(Claim{TaskID: "task-1", AgentID: "coder-2", Kind: ClaimKindDoer, GrantedAt: granted})

	if len(s.Claims) != 3 {
		t.Fatalf("len(Claims) = %d, want 3 (replace, not append)", len(s.Claims))
	}
	c := s.FindClaim("task-1", ClaimKindDoer)
	if c == nil || c.AgentID != "coder-2" {
		t.Fatalf("FindClaim after re-grant = %+v, want coder-2", c)
	}
	if !c.GrantedAt.Equal(granted) {
		t.Errorf("GrantedAt = %v, want %v", c.GrantedAt, granted)
	}
	if c.ExpiresAt != nil {
		t.Errorf("ExpiresAt = %v, want nil (fully replaced)", c.ExpiresAt)
	}
	// The reviewer claim on the same task must be untouched.
	if r := s.FindClaim("task-1", ClaimKindReviewer); r == nil || r.AgentID != "reviewer-1" {
		t.Errorf("reviewer claim disturbed by doer re-grant: %+v", r)
	}
}

func TestReleaseClaimRecord(t *testing.T) {
	s := claimTestState()

	if !s.ReleaseClaimRecord("task-1", ClaimKindDoer) {
		t.Error("ReleaseClaimRecord(task-1, doer) = false, want true")
	}
	if len(s.Claims) != 2 {
		t.Fatalf("len(Claims) = %d, want 2", len(s.Claims))
	}
	if s.FindClaim("task-1", ClaimKindDoer) != nil {
		t.Error("doer claim still present after release")
	}
	if s.FindClaim("task-1", ClaimKindReviewer) == nil {
		t.Error("reviewer claim removed by doer release")
	}

	// Releasing an absent claim is a no-op returning false.
	if s.ReleaseClaimRecord("task-1", ClaimKindDoer) {
		t.Error("second ReleaseClaimRecord = true, want false")
	}
	if s.ReleaseClaimRecord("missing-task", ClaimKindReviewer) {
		t.Error("ReleaseClaimRecord(missing-task) = true, want false")
	}
}
