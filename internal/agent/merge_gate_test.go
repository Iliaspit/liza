package agent

import (
	"testing"

	"github.com/liza-mas/liza/internal/models"
)

// TestMergeGateDiversityAchieved verifies that when provider-diversity is "preferred"
// and task approvals come from different providers, merge proceeds with
// diversity_achieved: true in the Extra map.
func TestMergeGateDiversityAchieved(t *testing.T) {
	task := &models.Task{
		ID:       "task-1",
		RolePair: "coding-pair",
		Approvals: []models.Approval{
			{Agent: "code-reviewer-1", Provider: "claude"},
			{Agent: "code-reviewer-2", Provider: "codex"},
		},
	}

	agents := map[string]models.Agent{
		"code-reviewer-1": {Role: "code-reviewer", Provider: "claude"},
		"code-reviewer-2": {Role: "code-reviewer", Provider: "codex"},
	}

	result := evaluateMergeGate(mergeGateInput{
		task:              task,
		agents:            agents,
		effectiveQuorum:   2,
		providerDiversity: "preferred",
		reviewerRole:      "code-reviewer",
	})

	if !result.proceed {
		t.Fatalf("expected proceed=true, got false (reason: %s)", result.skipReason)
	}
	if result.extra == nil {
		t.Fatal("expected extra to be non-nil")
	}
	if v, ok := result.extra["diversity_achieved"]; !ok || v != true {
		t.Errorf("expected diversity_achieved=true, got %v", result.extra)
	}
}

// TestMergeGateDiversityNotAchievable verifies that when provider-diversity is
// "preferred" but all registered reviewers share the same provider, merge proceeds
// with diversity_not_achievable: true and a collision-free diversity_reason.
func TestMergeGateDiversityNotAchievable(t *testing.T) {
	task := &models.Task{
		ID:       "task-1",
		RolePair: "coding-pair",
		Approvals: []models.Approval{
			{Agent: "code-reviewer-1", Provider: "claude"},
			{Agent: "code-reviewer-2", Provider: "claude"},
		},
	}

	agents := map[string]models.Agent{
		"code-reviewer-1": {Role: "code-reviewer", Provider: "claude"},
		"code-reviewer-2": {Role: "code-reviewer", Provider: "claude"},
	}

	result := evaluateMergeGate(mergeGateInput{
		task:              task,
		agents:            agents,
		effectiveQuorum:   2,
		providerDiversity: "preferred",
		reviewerRole:      "code-reviewer",
	})

	if !result.proceed {
		t.Fatalf("expected proceed=true, got false (reason: %s)", result.skipReason)
	}
	if result.extra == nil {
		t.Fatal("expected extra to be non-nil")
	}
	if v, ok := result.extra["diversity_not_achievable"]; !ok || v != true {
		t.Errorf("expected diversity_not_achievable=true, got %v", result.extra)
	}
	reason, ok := result.extra["diversity_reason"]
	if !ok {
		t.Fatal("expected diversity_reason in extra")
	}
	reasonStr, ok := reason.(string)
	if !ok || reasonStr == "" {
		t.Errorf("expected non-empty diversity_reason string, got %v", reason)
	}
	if _, ok := result.extra["reason"]; ok {
		t.Errorf("expected inline reason key to be absent, got %v", result.extra)
	}
}

// TestMergeGateQuorumDefenseInDepth verifies that the defense-in-depth check
// prevents merge when approval count is below the effective quorum.
func TestMergeGateQuorumDefenseInDepth(t *testing.T) {
	task := &models.Task{
		ID:       "task-1",
		RolePair: "coding-pair",
		Approvals: []models.Approval{
			{Agent: "code-reviewer-1", Provider: "claude"},
		},
	}

	agents := map[string]models.Agent{
		"code-reviewer-1": {Role: "code-reviewer", Provider: "claude"},
		"code-reviewer-2": {Role: "code-reviewer", Provider: "codex"},
	}

	result := evaluateMergeGate(mergeGateInput{
		task:              task,
		agents:            agents,
		effectiveQuorum:   2, // quorum=2 but only 1 approval
		providerDiversity: "preferred",
		reviewerRole:      "code-reviewer",
	})

	if result.proceed {
		t.Fatal("expected proceed=false for quorum defense-in-depth, got true")
	}
	if result.skipReason == "" {
		t.Error("expected non-empty skipReason")
	}
}

// TestMergeGateDiversityNotConfigured verifies that when provider-diversity is not
// configured (empty string), merge proceeds without diversity fields in Extra.
func TestMergeGateDiversityNotConfigured(t *testing.T) {
	task := &models.Task{
		ID:       "task-1",
		RolePair: "coding-pair",
		Approvals: []models.Approval{
			{Agent: "code-reviewer-1", Provider: "claude"},
		},
	}

	agents := map[string]models.Agent{
		"code-reviewer-1": {Role: "code-reviewer", Provider: "claude"},
	}

	result := evaluateMergeGate(mergeGateInput{
		task:              task,
		agents:            agents,
		effectiveQuorum:   1,
		providerDiversity: "", // not configured
		reviewerRole:      "code-reviewer",
	})

	if !result.proceed {
		t.Fatalf("expected proceed=true, got false (reason: %s)", result.skipReason)
	}
	// Extra should be nil — no diversity fields when not configured
	if result.extra != nil {
		t.Errorf("expected nil extra when diversity not configured, got %v", result.extra)
	}
}

// TestMergeGateDiversityNotMet verifies that when provider-diversity is "preferred",
// different providers exist in the pool, but the task approvals don't have diversity,
// merge proceeds with diversity_not_met: true.
func TestMergeGateDiversityNotMet(t *testing.T) {
	task := &models.Task{
		ID:       "task-1",
		RolePair: "coding-pair",
		Approvals: []models.Approval{
			{Agent: "code-reviewer-1", Provider: "claude"},
			{Agent: "code-reviewer-3", Provider: "claude"},
		},
	}

	// Pool has diversity (claude + codex) but approvals are both claude
	agents := map[string]models.Agent{
		"code-reviewer-1": {Role: "code-reviewer", Provider: "claude"},
		"code-reviewer-2": {Role: "code-reviewer", Provider: "codex"},
		"code-reviewer-3": {Role: "code-reviewer", Provider: "claude"},
	}

	result := evaluateMergeGate(mergeGateInput{
		task:              task,
		agents:            agents,
		effectiveQuorum:   2,
		providerDiversity: "preferred",
		reviewerRole:      "code-reviewer",
	})

	if !result.proceed {
		t.Fatalf("expected proceed=true, got false (reason: %s)", result.skipReason)
	}
	if result.extra == nil {
		t.Fatal("expected extra to be non-nil")
	}
	if v, ok := result.extra["diversity_not_met"]; !ok || v != true {
		t.Errorf("expected diversity_not_met=true, got %v", result.extra)
	}
}
