package statevalidate

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/liza-mas/liza/internal/models"
)

func claimsTestState() *models.State {
	coder := "coder-1"
	reviewer := "reviewer-1"
	return &models.State{
		Tasks: []models.Task{
			{ID: "task-1", Status: "IMPLEMENTING_CODE", AssignedTo: &coder},
			{ID: "task-2", Status: "REVIEWING_CODE", ReviewingBy: &reviewer},
			{ID: "task-3", Status: "READY"},
		},
		Agents: map[string]models.Agent{},
	}
}

func TestValidateClaims_AgreementProducesNoWarning(t *testing.T) {
	state := claimsTestState()
	state.Claims = []models.Claim{
		{TaskID: "task-1", AgentID: "coder-1", Kind: models.ClaimKindDoer, GrantedAt: time.Now().UTC()},
		{TaskID: "task-2", AgentID: "reviewer-1", Kind: models.ClaimKindReviewer, GrantedAt: time.Now().UTC()},
	}

	var warnings bytes.Buffer
	if err := validateClaims(state, &warnings); err != nil {
		t.Fatalf("validateClaims() error: %v", err)
	}
	if warnings.Len() != 0 {
		t.Errorf("expected no warnings, got %q", warnings.String())
	}
}

func TestValidateClaims_AbsentClaimForOwnedTaskIsSilent(t *testing.T) {
	// Old state files have no claims; ownership without a claim must neither
	// error nor warn.
	state := claimsTestState()

	var warnings bytes.Buffer
	if err := validateClaims(state, &warnings); err != nil {
		t.Fatalf("validateClaims() error: %v", err)
	}
	if warnings.Len() != 0 {
		t.Errorf("expected no warnings, got %q", warnings.String())
	}
}

func TestValidateClaims_ContradictionWarns(t *testing.T) {
	tests := []struct {
		name         string
		claim        models.Claim
		wantContains string
	}{
		{
			name:         "doer claim disagrees with assigned_to",
			claim:        models.Claim{TaskID: "task-1", AgentID: "coder-9", Kind: models.ClaimKindDoer},
			wantContains: "doer claim for task task-1 held by coder-9 but legacy assigned_to is coder-1",
		},
		{
			name:         "doer claim on unassigned task",
			claim:        models.Claim{TaskID: "task-3", AgentID: "coder-1", Kind: models.ClaimKindDoer},
			wantContains: "doer claim for task task-3 held by coder-1 but legacy assigned_to is unset",
		},
		{
			name:         "reviewer claim disagrees with reviewing_by",
			claim:        models.Claim{TaskID: "task-2", AgentID: "reviewer-9", Kind: models.ClaimKindReviewer},
			wantContains: "reviewer claim for task task-2 held by reviewer-9 but legacy reviewing_by is reviewer-1",
		},
		{
			name:         "reviewer claim on task without reviewer",
			claim:        models.Claim{TaskID: "task-1", AgentID: "reviewer-1", Kind: models.ClaimKindReviewer},
			wantContains: "reviewer claim for task task-1 held by reviewer-1 but legacy reviewing_by is unset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := claimsTestState()
			state.Claims = []models.Claim{tt.claim}

			var warnings bytes.Buffer
			if err := validateClaims(state, &warnings); err != nil {
				t.Fatalf("validateClaims() error: %v (contradictions must warn, not error)", err)
			}
			if !strings.Contains(warnings.String(), tt.wantContains) {
				t.Errorf("warnings = %q, want to contain %q", warnings.String(), tt.wantContains)
			}
		})
	}
}

func TestValidateClaims_MissingTaskIsError(t *testing.T) {
	state := claimsTestState()
	state.Claims = []models.Claim{
		{TaskID: "ghost-task", AgentID: "coder-1", Kind: models.ClaimKindDoer},
	}

	var warnings bytes.Buffer
	err := validateClaims(state, &warnings)
	if err == nil {
		t.Fatal("expected error for claim referencing non-existent task, got nil")
	}
	if !strings.Contains(err.Error(), "ghost-task") {
		t.Errorf("error = %q, want to mention ghost-task", err.Error())
	}
}
